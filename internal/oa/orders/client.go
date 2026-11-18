package orders

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

var (
	errUnavailable = errors.New("订单数据暂时不可用")
	errDenied      = errors.New("订单数据源拒绝访问")
	errBusy        = errors.New("订单查询繁忙")
	errTimeout     = errors.New("订单查询超时")
)

type client struct {
	base     *url.URL
	password string
	http     *http.Client
	slots    chan struct{}
	sequence atomic.Uint64
}

func newClient() *client {
	endpoint := os.Getenv("OA_CLICKHOUSE_URL")
	if endpoint == "" {
		endpoint = "http://100.97.209.121:8123"
	}
	base, err := url.Parse(endpoint)
	password := os.Getenv("OPS_READER_PASSWORD")
	if err != nil || base.Host == "" || (base.Scheme != "http" && base.Scheme != "https") || base.User != nil || password == "" {
		return nil
	}
	transport := http.DefaultTransport.(*http.Transport).Clone()
	transport.Proxy = nil
	transport.DialContext = (&net.Dialer{Timeout: 2 * time.Second, KeepAlive: 30 * time.Second}).DialContext
	transport.MaxIdleConnsPerHost = 8
	transport.MaxConnsPerHost = 8
	transport.IdleConnTimeout = 90 * time.Second
	return &client{base: base, password: password, slots: make(chan struct{}, 8), http: &http.Client{Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}}
}

type row = map[string]json.RawMessage

func (c *client) request(ctx context.Context, query string, values url.Values) ([]byte, error) {
	if c == nil {
		return nil, errUnavailable
	}
	ctx, cancel := context.WithTimeout(ctx, 12*time.Second)
	defer cancel()
	select {
	case c.slots <- struct{}{}:
		defer func() { <-c.slots }()
	case <-ctx.Done():
		return nil, errTimeout
	}
	endpoint := *c.base
	params := make(url.Values, len(values)+2)
	for name, value := range values {
		params[name] = value
	}
	method := http.MethodPost
	if query == "" {
		endpoint.Path = "/ping"
		method = http.MethodGet
	} else {
		params.Set("database", "jst")
		params.Set("query_id", fmt.Sprintf("oa-%d-%d", time.Now().UnixNano(), c.sequence.Add(1)))
	}
	endpoint.RawQuery = params.Encode()
	for attempt := 0; attempt < 2; attempt++ {
		request, err := http.NewRequestWithContext(ctx, method, endpoint.String(), strings.NewReader(query))
		if err != nil {
			return nil, errUnavailable
		}
		request.Header.Set("X-ClickHouse-User", "ops_reader")
		request.Header.Set("X-ClickHouse-Key", c.password)
		response, err := c.http.Do(request)
		if err != nil {
			var dialErr *net.OpError
			if attempt == 0 && errors.As(err, &dialErr) && dialErr.Op == "dial" && ctx.Err() == nil {
				continue
			}
			if errors.Is(err, context.DeadlineExceeded) {
				return nil, errTimeout
			}
			return nil, errUnavailable
		}
		body, err := io.ReadAll(io.LimitReader(response.Body, (8<<20)+1))
		if err == nil && len(body) > 8<<20 {
			_, _ = io.Copy(io.Discard, response.Body)
		}
		response.Body.Close()
		if errors.Is(err, context.DeadlineExceeded) {
			return nil, errTimeout
		}
		if err != nil || len(body) > 8<<20 {
			return nil, errUnavailable
		}
		switch {
		case response.StatusCode == 401 || response.StatusCode == 403:
			return nil, errDenied
		case response.StatusCode == 429:
			return nil, errBusy
		case response.StatusCode == 200 && !strings.Contains(string(body), "DB::Exception:"):
			return body, nil
		}
		switch message := string(body); {
		case strings.Contains(message, "(ACCESS_DENIED)"), strings.Contains(message, "(AUTHENTICATION_FAILED)"):
			return nil, errDenied
		case strings.Contains(message, "(TOO_MANY_SIMULTANEOUS_QUERIES)"):
			return nil, errBusy
		case strings.Contains(message, "(TIMEOUT_EXCEEDED)"):
			return nil, errTimeout
		}
		return nil, errUnavailable
	}
	return nil, errUnavailable
}

func (c *client) rows(ctx context.Context, query string, params url.Values) ([]row, error) {
	body, err := c.request(ctx, query+" FORMAT JSON", params)
	if err != nil {
		return nil, err
	}
	var result struct {
		Data []row `json:"data"`
	}
	if json.Unmarshal(body, &result) != nil || result.Data == nil {
		return nil, errUnavailable
	}
	return result.Data, nil
}

func (c *client) keepAlive() {
	if c == nil {
		return
	}
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			_, _ = c.request(context.Background(), "", nil)
		}
	}()
}
