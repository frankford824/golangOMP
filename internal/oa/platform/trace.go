package platform

import (
	"context"
	"log"
	"net/http"
	"regexp"
	"strings"

	"github.com/google/uuid"
)

const TraceHeader = "X-Trace-ID"
const ErrorFormatHeader = "X-OA-Error-Format"

type traceKey struct{}

var tracePattern = regexp.MustCompile("^[0-9a-f]{32}$")

// TraceID returns the correlation ID assigned to the current OA request.
func TraceID(ctx context.Context) string {
	trace, _ := ctx.Value(traceKey{}).(string)
	return trace
}

func traceContext(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, traceKey{}, id)
}

func Trace(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		id := r.Header.Get(TraceHeader)
		if !tracePattern.MatchString(id) {
			id = uuid.New().String()
		}
		w.Header().Set(TraceHeader, id)
		w.Header().Add("Vary", ErrorFormatHeader)
		writer := &traceResponse{ResponseWriter: w, v2: strings.EqualFold(r.Header.Get(ErrorFormatHeader), "v2")}
		defer func() {
			if recovered := recover(); recovered != nil {
				log.Printf("oa_request_panic trace_id=%s", id)
				if writer.status != 0 {
					panic(http.ErrAbortHandler)
				}
				Error(writer, http.StatusInternalServerError, "OA 服务暂时不可用")
			}
			if writer.status >= http.StatusBadRequest {
				log.Printf("oa_request_failed trace_id=%s status=%d", id, writer.status)
			}
		}()
		next.ServeHTTP(writer, r.WithContext(traceContext(r.Context(), id)))
	})
}

type traceResponse struct {
	http.ResponseWriter
	status     int
	v2         bool
	normalized bool
}

func (w *traceResponse) Unwrap() http.ResponseWriter { return w.ResponseWriter }

func (w *traceResponse) WriteHeader(status int) {
	if w.status == 0 && (status >= 200 || status == http.StatusSwitchingProtocols) {
		w.status = status
		// The stdlib router emits plain-text 404/405. Keep its status and Allow header.
		if w.v2 && status >= http.StatusBadRequest && w.Header().Get("Content-Type") != "application/json; charset=utf-8" {
			w.Header().Del("Content-Length")
			writeEnvelope(w.ResponseWriter, status, "", "", w.Header().Get(TraceHeader))
			w.normalized = true
			return
		}
	}
	if !w.normalized {
		w.ResponseWriter.WriteHeader(status)
	}
}

func (w *traceResponse) Write(body []byte) (int, error) {
	if w.status == 0 {
		w.status = http.StatusOK
	}
	if w.normalized {
		return len(body), nil
	}
	return w.ResponseWriter.Write(body)
}

// ErrorWithCode keeps the legacy error string and optionally preserves a business code.
func ErrorWithCode(w http.ResponseWriter, status int, code, message string) {
	id := w.Header().Get(TraceHeader)
	if rw, ok := w.(*traceResponse); ok && rw.v2 {
		writeEnvelope(w, status, code, message, id)
		return
	}
	body := map[string]string{"error": message}
	if code != "" {
		body["code"] = code
	}
	if id != "" {
		body["trace_id"] = id
	}
	write(w, status, body)
}

func writeEnvelope(w http.ResponseWriter, status int, code, message, id string) {
	fallbackCode, fallbackMessage := "BAD_REQUEST", "请求参数有误，请检查后重试"
	switch status {
	case 401:
		fallbackCode, fallbackMessage = "UNAUTHENTICATED", "登录状态已失效，请重新登录"
	case 403:
		fallbackCode, fallbackMessage = "FORBIDDEN", "暂无权限执行该操作"
	case 404:
		fallbackCode, fallbackMessage = "NOT_FOUND", "请求的资源不存在"
	case 405:
		fallbackCode, fallbackMessage = "METHOD_NOT_ALLOWED", "当前入口不支持该操作"
	case 408, 504:
		fallbackCode, fallbackMessage = "DEADLINE_EXCEEDED", "请求超时，请稍后重试"
	case 409:
		fallbackCode, fallbackMessage = "CONFLICT", "数据已发生变化，请刷新后重试"
	case 413:
		fallbackCode, fallbackMessage = "PAYLOAD_TOO_LARGE", "上传内容超过大小限制"
	case 422:
		fallbackCode, fallbackMessage = "VALIDATION_ERROR", "提交的信息不符合要求"
	case 429:
		fallbackCode, fallbackMessage = "RESOURCE_EXHAUSTED", "请求过于频繁，请稍后重试"
	default:
		if status >= 500 {
			fallbackCode, fallbackMessage = "INTERNAL_ERROR", "服务暂时不可用，请稍后重试"
		}
	}
	if code == "" {
		code = fallbackCode
	}
	if message == "" {
		message = fallbackMessage
	}
	write(w, status, map[string]any{"error": map[string]string{"code": code, "message": message, "trace_id": id}})
}
