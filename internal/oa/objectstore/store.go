package objectstore

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

const prefix = "/oa-objects/"
const maxObject = 2 << 30
const maxStorage = 20 << 30

type store struct {
	root, keyID, secret string
	mu                  sync.Mutex
}

func New(root, keyID, secret string) http.Handler {
	if !filepath.IsAbs(root) || len(secret) < 32 || keyID == "" {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Error(w, "storage unavailable", 503) })
	}
	return &store{root: root, keyID: keyID, secret: secret}
}

func validKey(key string) bool {
	return key != "" && len(key) <= 1024 && path.Clean(key) == key && !strings.HasPrefix(key, "/") && key != "." && key != ".." && !strings.HasPrefix(key, "../") && !strings.ContainsAny(key, "\\\x00\r\n")
}

func digest(key string) string { return fmt.Sprintf("%x", sha256.Sum256([]byte(key))) }

func (s *store) authorized(r *http.Request, key string, q url.Values) bool {
	resource := prefix + key
	extras := []string{}
	for name, values := range q {
		if len(values) != 1 {
			return false
		}
		switch name {
		case "uploads":
			if values[0] != "" {
				return false
			}
			extras = append(extras, name)
		case "uploadId", "partNumber", "response-content-disposition", "response-cache-control", "x-oss-process":
			extras = append(extras, name+"="+values[0])
		case "OSSAccessKeyId", "Expires", "Signature":
		default:
			return false
		}
	}
	sort.Strings(extras)
	if len(extras) > 0 {
		resource += "?" + strings.Join(extras, "&")
	}
	timestamp := r.Header.Get("Date")
	signature := strings.TrimPrefix(r.Header.Get("Authorization"), "OSS "+s.keyID+":")
	if q.Has("Signature") {
		if r.Method != http.MethodGet && r.Method != http.MethodPut {
			return false
		}
		if r.Method == http.MethodPut && (r.Header.Get("x-oss-copy-source") != "" || q.Has("uploads") || q.Has("uploadId") != q.Has("partNumber")) {
			return false
		}
		if q.Get("OSSAccessKeyId") != s.keyID || r.Header.Get("Authorization") != "" {
			return false
		}
		timestamp = q.Get("Expires")
		expiry, err := strconv.ParseInt(timestamp, 10, 64)
		if err != nil || expiry < time.Now().Unix() || expiry > time.Now().Add(24*time.Hour).Unix() {
			return false
		}
		signature = q.Get("Signature")
	} else {
		if !strings.HasPrefix(r.Header.Get("Authorization"), "OSS "+s.keyID+":") {
			return false
		}
		date, err := http.ParseTime(timestamp)
		if err != nil || time.Since(date) > 15*time.Minute || time.Until(date) > 15*time.Minute {
			return false
		}
	}
	headers := []string{}
	for key, values := range r.Header {
		name := strings.ToLower(key)
		if strings.HasPrefix(name, "x-oss-") {
			headers = append(headers, name+":"+strings.TrimSpace(strings.Join(values, ","))+"\n")
		}
	}
	sort.Strings(headers)
	msg := strings.Join([]string{r.Method, r.Header.Get("Content-MD5"), r.Header.Get("Content-Type"), timestamp, ""}, "\n") + strings.Join(headers, "") + resource
	mac := hmac.New(sha1.New, []byte(s.secret))
	_, _ = io.WriteString(mac, msg)
	expected := base64.StdEncoding.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expected))
}

func (s *store) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Header().Set("Content-Security-Policy", "sandbox; default-src 'none'; frame-ancestors 'self'")
	w.Header().Set("Referrer-Policy", "no-referrer")
	w.Header().Set("Cache-Control", "private, no-store")
	key, ok := strings.CutPrefix(r.URL.Path, prefix)
	q, err := url.ParseQuery(r.URL.RawQuery)
	if err != nil || !ok || !validKey(key) || !s.authorized(r, key, q) {
		http.Error(w, "access denied", 403)
		return
	}
	dest := filepath.Join(s.root, "objects", digest(key))
	if r.Method == http.MethodGet || r.Method == http.MethodHead {
		f, err := os.Open(dest)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		info, err := f.Stat()
		if err != nil {
			http.Error(w, "storage unavailable", 503)
			return
		}
		contentType := "application/octet-stream"
		var prefixBytes [512]byte
		if count, _ := f.ReadAt(prefixBytes[:], 0); count > 0 {
			detected := http.DetectContentType(prefixBytes[:count])
			switch detected {
			case "image/png", "image/jpeg", "image/gif", "image/webp", "image/bmp":
				contentType = detected
			}
		}
		w.Header().Set("Content-Type", contentType)
		disposition := q.Get("response-content-disposition")
		if contentType == "application/octet-stream" && !strings.HasPrefix(disposition, "attachment;") {
			disposition = "attachment"
		}
		if disposition != "" && !strings.ContainsAny(disposition, "\r\n") {
			w.Header().Set("Content-Disposition", disposition)
		}
		http.ServeContent(w, r, key, info.ModTime(), f)
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var uploadPath string
	if id := q.Get("uploadId"); id != "" {
		raw, e := hex.DecodeString(id)
		if e != nil || len(raw) != 16 || len(id) != 32 {
			http.NotFound(w, r)
			return
		}
		uploadPath = filepath.Join(s.root, "uploads", digest(key), id)
		info, e := os.Stat(uploadPath)
		if e != nil || !info.IsDir() {
			http.NotFound(w, r)
			return
		}
	}
	switch r.Method {
	case http.MethodPut:
		if uploadPath != "" {
			part, e := strconv.Atoi(q.Get("partNumber"))
			if e != nil || part < 1 || part > 10000 {
				http.Error(w, "invalid part", 400)
				return
			}
			dest = filepath.Join(uploadPath, strconv.Itoa(part))
		} else if q.Has("partNumber") {
			http.Error(w, "invalid part", 400)
			return
		}
		source := r.Body
		if copySource := r.Header.Get("x-oss-copy-source"); copySource != "" {
			decoded, e := url.PathUnescape(copySource)
			src, matched := strings.CutPrefix(decoded, prefix)
			if e != nil || !matched || !validKey(src) || uploadPath != "" {
				http.Error(w, "invalid source", 400)
				return
			}
			f, e := os.Open(filepath.Join(s.root, "objects", digest(src)))
			if e != nil {
				http.NotFound(w, r)
				return
			}
			defer f.Close()
			source = f
		}
		etag, e := s.write(dest, source)
		if e != nil {
			writeError(w, e)
			return
		}
		w.Header().Set("ETag", etag)
		w.WriteHeader(200)
	case http.MethodDelete:
		var e error
		if uploadPath != "" {
			e = os.RemoveAll(uploadPath)
		} else {
			e = os.Remove(dest)
		}
		if e != nil && !errors.Is(e, fs.ErrNotExist) {
			writeError(w, e)
			return
		}
		w.WriteHeader(204)
	case http.MethodPost:
		if q.Has("uploads") && uploadPath == "" {
			var raw [16]byte
			if _, e := rand.Read(raw[:]); e != nil {
				writeError(w, e)
				return
			}
			id := hex.EncodeToString(raw[:])
			if e := os.MkdirAll(filepath.Join(s.root, "uploads", digest(key), id), 0700); e != nil {
				writeError(w, e)
				return
			}
			w.Header().Set("Content-Type", "application/xml")
			_ = xml.NewEncoder(w).Encode(struct {
				XMLName  xml.Name `xml:"InitiateMultipartUploadResult"`
				Bucket   string
				Key      string
				UploadID string `xml:"UploadId"`
			}{Bucket: "oa-objects", Key: key, UploadID: id})
		} else if uploadPath != "" && !q.Has("uploads") {
			s.complete(w, r, dest, uploadPath)
		} else {
			http.Error(w, "invalid multipart action", 400)
		}
	default:
		http.Error(w, "method not allowed", 405)
	}
}

var errLimit = errors.New("object or storage limit exceeded")

func writeError(w http.ResponseWriter, err error) {
	if errors.Is(err, errLimit) {
		http.Error(w, errLimit.Error(), 413)
		return
	}
	http.Error(w, "storage unavailable", 503)
}

func (s *store) write(dest string, reader io.Reader) (string, error) {
	if err := os.MkdirAll(filepath.Dir(dest), 0700); err != nil {
		return "", err
	}
	remaining := int64(maxStorage)
	err := filepath.WalkDir(s.root, func(name string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.Type().IsRegular() {
			return nil
		}
		info, e := d.Info()
		if e != nil {
			return e
		}
		remaining -= info.Size()
		return nil
	})
	if err != nil {
		return "", err
	}
	limit := min(int64(maxObject), remaining)
	if limit <= 0 {
		return "", errLimit
	}
	tmp, e := os.CreateTemp(filepath.Dir(dest), ".incoming-")
	if e != nil {
		return "", e
	}
	defer os.Remove(tmp.Name())
	hash := sha256.New()
	size, e := io.Copy(io.MultiWriter(tmp, hash), io.LimitReader(reader, limit+1))
	closeErr := tmp.Close()
	if e != nil {
		return "", e
	}
	if closeErr != nil {
		return "", closeErr
	}
	if size > limit {
		return "", errLimit
	}
	if e = os.Rename(tmp.Name(), dest); e != nil {
		return "", e
	}
	return fmt.Sprintf("\"%x\"", hash.Sum(nil)), nil
}

type multipartReader struct {
	directory string
	parts     []struct {
		Number int    `xml:"PartNumber"`
		ETag   string `xml:"ETag"`
	}
	current *os.File
	hash    io.Writer
	verify  func() string
}

func (m *multipartReader) Read(p []byte) (int, error) {
	for len(m.parts) > 0 {
		if m.current == nil {
			file, err := os.Open(filepath.Join(m.directory, strconv.Itoa(m.parts[0].Number)))
			if err != nil {
				return 0, err
			}
			m.current = file
			hash := sha256.New()
			m.hash = hash
			m.verify = func() string { return hex.EncodeToString(hash.Sum(nil)) }
		}
		n, err := m.current.Read(p)
		if n > 0 {
			_, _ = m.hash.Write(p[:n])
		}
		if err == io.EOF {
			_ = m.current.Close()
			m.current = nil
			if !hmac.Equal([]byte(m.verify()), []byte(strings.Trim(m.parts[0].ETag, "\""))) {
				return n, errPart
			}
			m.parts = m.parts[1:]
			if n > 0 {
				return n, nil
			}
			continue
		}
		return n, err
	}
	return 0, io.EOF
}

var errPart = errors.New("part missing or checksum mismatch")

func (s *store) complete(w http.ResponseWriter, r *http.Request, dest, uploadPath string) {
	var body struct {
		XMLName xml.Name `xml:"CompleteMultipartUpload"`
		Parts   []struct {
			Number int    `xml:"PartNumber"`
			ETag   string `xml:"ETag"`
		} `xml:"Part"`
	}
	if xml.NewDecoder(http.MaxBytesReader(w, r.Body, 2<<20)).Decode(&body) != nil || len(body.Parts) == 0 || len(body.Parts) > 10000 {
		http.Error(w, "invalid part list", 400)
		return
	}
	for i, part := range body.Parts {
		if part.Number != i+1 || part.ETag == "" {
			http.Error(w, "invalid part list", 400)
			return
		}
	}
	reader := &multipartReader{directory: uploadPath, parts: body.Parts}
	etag, err := s.write(dest, reader)
	if reader.current != nil {
		_ = reader.current.Close()
	}
	if errors.Is(err, fs.ErrNotExist) || errors.Is(err, errPart) {
		http.Error(w, errPart.Error(), 400)
		return
	}
	if err != nil {
		writeError(w, err)
		return
	}
	_ = os.RemoveAll(uploadPath)
	w.Header().Set("ETag", etag)
	w.WriteHeader(200)
}
