package handler

import (
	"encoding/json"
	"net/http"
	"net/url"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"workflow/domain"
	analyticssvc "workflow/service/analytics"
)

type analyticsMCPRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type analyticsMCPResponse struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id,omitempty"`
	Result  interface{}     `json:"result,omitempty"`
	Error   interface{}     `json:"error,omitempty"`
}

func (h *AIChatHandler) AnalyticsMCPGet(c *gin.Context) {
	c.Header("Allow", http.MethodPost)
	c.Status(http.StatusMethodNotAllowed)
}

func (h *AIChatHandler) AnalyticsMCPPost(c *gin.Context) {
	if !validMCPOrigin(c.Request) {
		c.AbortWithStatus(http.StatusForbidden)
		return
	}
	if h == nil || h.analytics == nil {
		c.JSON(http.StatusServiceUnavailable, analyticsMCPResponse{JSONRPC: "2.0", Error: map[string]interface{}{"code": -32603, "message": "analytics MCP is unavailable"}})
		return
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 1<<20)
	var request analyticsMCPRequest
	if err := c.ShouldBindJSON(&request); err != nil || request.JSONRPC != "2.0" || strings.TrimSpace(request.Method) == "" {
		c.JSON(http.StatusBadRequest, analyticsMCPResponse{JSONRPC: "2.0", Error: map[string]interface{}{"code": -32700, "message": "invalid JSON-RPC request"}})
		return
	}
	actor, _ := domain.RequestActorFromContext(c.Request.Context())
	c.Header("MCP-Protocol-Version", analyticssvc.ProtocolVersion)
	switch request.Method {
	case "initialize":
		c.Header("Mcp-Session-Id", uuid.NewString())
		h.respondMCP(c, request.ID, map[string]interface{}{
			"protocolVersion": analyticssvc.ProtocolVersion,
			"capabilities":    map[string]interface{}{"tools": map[string]interface{}{"listChanged": false}},
			"serverInfo":      map[string]interface{}{"name": "yongbo-analytics", "title": "永箔只读数据分析", "version": "1.0.0"},
			"instructions":    "所有指标查询均为只读、受当前账号数据范围约束，禁止传入SQL。",
		})
	case "notifications/initialized":
		c.Status(http.StatusAccepted)
	case "ping":
		h.respondMCP(c, request.ID, map[string]interface{}{})
	case "tools/list":
		tools, appErr := h.analytics.Tools(actor)
		if appErr != nil {
			h.respondMCPError(c, request.ID, -32603, appErr.Message)
			return
		}
		h.respondMCP(c, request.ID, map[string]interface{}{"tools": tools})
	case "tools/call":
		var params struct {
			Name      string                 `json:"name"`
			Arguments map[string]interface{} `json:"arguments"`
		}
		if err := json.Unmarshal(request.Params, &params); err != nil || strings.TrimSpace(params.Name) == "" {
			h.respondMCPError(c, request.ID, -32602, "tool name and arguments are required")
			return
		}
		output, appErr := h.analytics.Call(c.Request.Context(), actor, params.Name, params.Arguments)
		if appErr != nil {
			h.respondMCP(c, request.ID, map[string]interface{}{
				"content": []map[string]interface{}{{"type": "text", "text": appErr.Message}},
				"isError": true,
			})
			return
		}
		h.respondMCP(c, request.ID, map[string]interface{}{
			"content":           []map[string]interface{}{{"type": "text", "text": output.Text}},
			"structuredContent": output.Structured,
			"isError":           false,
		})
	default:
		h.respondMCPError(c, request.ID, -32601, "method not found")
	}
}

func (h *AIChatHandler) respondMCP(c *gin.Context, id json.RawMessage, result interface{}) {
	c.JSON(http.StatusOK, analyticsMCPResponse{JSONRPC: "2.0", ID: id, Result: result})
}

func (h *AIChatHandler) respondMCPError(c *gin.Context, id json.RawMessage, code int, message string) {
	c.JSON(http.StatusOK, analyticsMCPResponse{JSONRPC: "2.0", ID: id, Error: map[string]interface{}{"code": code, "message": message}})
}

func validMCPOrigin(request *http.Request) bool {
	origin := strings.TrimSpace(request.Header.Get("Origin"))
	if origin == "" {
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.Host == "" {
		return false
	}
	host := strings.TrimSpace(request.Header.Get("X-Forwarded-Host"))
	if host == "" {
		host = request.Host
	}
	return strings.EqualFold(parsed.Host, host)
}
