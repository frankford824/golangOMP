package wecombot

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync/atomic"
	"time"

	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

const (
	defaultWSURL     = "wss://openws.work.weixin.qq.com"
	defaultQueueSize = 200
)

type Config struct {
	Enabled       bool
	BotID         string
	Secret        string
	DefaultChatID string
	WSURL         string
	QueueSize     int
}

type Sender struct {
	cfg     Config
	logger  *zap.Logger
	queue   chan markdownMessage
	enabled bool
	seq     atomic.Uint64
}

type markdownMessage struct {
	chatID  string
	content string
}

type wsFrame struct {
	Cmd     string                 `json:"cmd,omitempty"`
	Headers map[string]interface{} `json:"headers,omitempty"`
	Body    map[string]interface{} `json:"body,omitempty"`
	ErrCode *int                   `json:"errcode,omitempty"`
	ErrMsg  string                 `json:"errmsg,omitempty"`
}

func NewSender(cfg Config, logger *zap.Logger) *Sender {
	if logger == nil {
		logger = zap.NewNop()
	}
	cfg.BotID = strings.TrimSpace(cfg.BotID)
	cfg.Secret = strings.TrimSpace(cfg.Secret)
	cfg.DefaultChatID = strings.TrimSpace(cfg.DefaultChatID)
	cfg.WSURL = strings.TrimSpace(cfg.WSURL)
	if cfg.WSURL == "" {
		cfg.WSURL = defaultWSURL
	}
	if cfg.QueueSize <= 0 {
		cfg.QueueSize = defaultQueueSize
	}
	enabled := cfg.Enabled && cfg.BotID != "" && cfg.Secret != "" && cfg.DefaultChatID != ""
	if cfg.Enabled && !enabled {
		logger.Warn("wecom aibot disabled because required config is incomplete")
	}
	return &Sender{cfg: cfg, logger: logger, queue: make(chan markdownMessage, cfg.QueueSize), enabled: enabled}
}

func (s *Sender) Enabled() bool {
	return s != nil && s.enabled
}

func (s *Sender) Start(ctx context.Context) bool {
	if !s.Enabled() {
		return false
	}
	go s.run(ctx)
	return true
}

func (s *Sender) EnqueueMarkdown(ctx context.Context, content string) error {
	if !s.Enabled() {
		return nil
	}
	msg := markdownMessage{chatID: s.cfg.DefaultChatID, content: strings.TrimSpace(content)}
	if msg.content == "" {
		return nil
	}
	select {
	case s.queue <- msg:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	default:
		return errors.New("wecom aibot queue is full")
	}
}

func (s *Sender) run(ctx context.Context) {
	backoff := time.Second
	for {
		if ctx.Err() != nil {
			return
		}
		err := s.runConnection(ctx)
		if ctx.Err() != nil {
			return
		}
		if err != nil {
			s.logger.Warn("wecom aibot connection ended", zap.Error(err), zap.Duration("retry_after", backoff))
		}
		select {
		case <-ctx.Done():
			return
		case <-time.After(backoff):
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
	}
}

func (s *Sender) runConnection(ctx context.Context) error {
	conn, err := s.connectAndSubscribe(ctx)
	if err != nil {
		return err
	}
	defer conn.Close()
	s.logger.Info("wecom aibot connected")

	frames := make(chan wsFrame, 16)
	readErr := make(chan error, 1)
	go readLoop(conn, frames, readErr)

	pending := map[string]markdownMessage{}
	pingTicker := time.NewTicker(30 * time.Second)
	defer pingTicker.Stop()
	for {
		select {
		case <-ctx.Done():
			return nil
		case err := <-readErr:
			return err
		case frame := <-frames:
			s.handleFrame(frame, pending)
		case msg := <-s.queue:
			reqID := s.nextReqID("aibot_send_msg")
			pending[reqID] = msg
			if err := conn.WriteJSON(wsFrame{
				Cmd:     "aibot_send_msg",
				Headers: map[string]interface{}{"req_id": reqID},
				Body: map[string]interface{}{
					"chatid":  msg.chatID,
					"msgtype": "markdown",
					"markdown": map[string]interface{}{
						"content": msg.content,
					},
				},
			}); err != nil {
				delete(pending, reqID)
				return err
			}
		case <-pingTicker.C:
			if err := conn.WriteJSON(wsFrame{
				Cmd:     "ping",
				Headers: map[string]interface{}{"req_id": s.nextReqID("ping")},
			}); err != nil {
				return err
			}
		}
	}
}

func (s *Sender) connectAndSubscribe(ctx context.Context) (*websocket.Conn, error) {
	conn, _, err := websocket.DefaultDialer.DialContext(ctx, s.cfg.WSURL, nil)
	if err != nil {
		return nil, err
	}
	reqID := s.nextReqID("aibot_subscribe")
	if err := conn.WriteJSON(wsFrame{
		Cmd:     "aibot_subscribe",
		Headers: map[string]interface{}{"req_id": reqID},
		Body: map[string]interface{}{
			"bot_id": s.cfg.BotID,
			"secret": s.cfg.Secret,
		},
	}); err != nil {
		conn.Close()
		return nil, err
	}
	deadline := time.Now().Add(10 * time.Second)
	if err := conn.SetReadDeadline(deadline); err != nil {
		conn.Close()
		return nil, err
	}
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			conn.Close()
			return nil, err
		}
		var frame wsFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue
		}
		if reqIDFromFrame(frame) != reqID {
			continue
		}
		if frame.ErrCode != nil && *frame.ErrCode != 0 {
			conn.Close()
			return nil, fmt.Errorf("wecom aibot subscribe failed: %d %s", *frame.ErrCode, frame.ErrMsg)
		}
		_ = conn.SetReadDeadline(time.Time{})
		return conn, nil
	}
}

func (s *Sender) handleFrame(frame wsFrame, pending map[string]markdownMessage) {
	reqID := reqIDFromFrame(frame)
	if reqID == "" {
		return
	}
	if _, ok := pending[reqID]; !ok {
		return
	}
	delete(pending, reqID)
	if frame.ErrCode != nil && *frame.ErrCode != 0 {
		s.logger.Warn("wecom aibot send failed", zap.String("req_id", reqID), zap.Int("errcode", *frame.ErrCode), zap.String("errmsg", frame.ErrMsg))
		return
	}
	s.logger.Debug("wecom aibot send ok", zap.String("req_id", reqID))
}

func (s *Sender) nextReqID(prefix string) string {
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), s.seq.Add(1))
}

func readLoop(conn *websocket.Conn, frames chan<- wsFrame, readErr chan<- error) {
	for {
		_, raw, err := conn.ReadMessage()
		if err != nil {
			readErr <- err
			return
		}
		var frame wsFrame
		if err := json.Unmarshal(raw, &frame); err != nil {
			continue
		}
		frames <- frame
	}
}

func reqIDFromFrame(frame wsFrame) string {
	if frame.Headers == nil {
		return ""
	}
	raw, ok := frame.Headers["req_id"]
	if !ok {
		return ""
	}
	return strings.TrimSpace(fmt.Sprint(raw))
}
