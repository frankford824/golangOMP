package externalassets

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

type AListClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type BFFClient struct {
	baseURL        string
	browserBaseURL string
	httpClient     *http.Client
}

func NewAListClient(baseURL, token string, timeout time.Duration) *AListClient {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	return &AListClient{
		baseURL:    strings.TrimRight(strings.TrimSpace(baseURL), "/"),
		token:      strings.TrimSpace(strings.ReplaceAll(token, "\r", "")),
		httpClient: &http.Client{Timeout: timeout},
	}
}

func NewBFFClient(baseURL, browserBaseURL string, timeout time.Duration) *BFFClient {
	if timeout <= 0 {
		timeout = 20 * time.Second
	}
	baseURL = strings.TrimRight(strings.TrimSpace(baseURL), "/")
	browserBaseURL = strings.TrimRight(strings.TrimSpace(browserBaseURL), "/")
	if browserBaseURL == "" {
		browserBaseURL = baseURL
	}
	return &BFFClient{
		baseURL:        baseURL,
		browserBaseURL: browserBaseURL,
		httpClient:     &http.Client{Timeout: timeout},
	}
}

func (c *AListClient) Enabled() bool {
	return c != nil && c.baseURL != "" && c.token != ""
}

func (c *BFFClient) Enabled() bool {
	return c != nil && c.baseURL != ""
}

type AListSearchItem struct {
	Parent   string    `json:"parent"`
	Name     string    `json:"name"`
	IsDir    bool      `json:"is_dir"`
	Size     int64     `json:"size"`
	Type     int       `json:"type"`
	Modified time.Time `json:"modified"`
}

type AListSearchResponse struct {
	Content []AListSearchItem `json:"content"`
	Total   int64             `json:"total"`
}

type AListListResponse struct {
	Content []AListSearchItem `json:"content"`
	Total   int64             `json:"total"`
}

type AListFileInfo struct {
	ID       string    `json:"id"`
	Path     string    `json:"path"`
	Name     string    `json:"name"`
	Size     int64     `json:"size"`
	IsDir    bool      `json:"is_dir"`
	Modified time.Time `json:"modified"`
	Created  time.Time `json:"created"`
	Sign     string    `json:"sign"`
	Thumb    string    `json:"thumb"`
	Type     int       `json:"type"`
	RawURL   string    `json:"raw_url"`
	Provider string    `json:"provider"`
}

func (c *AListClient) Search(ctx context.Context, parent, keyword string, page, perPage int) (*AListSearchResponse, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("alist client is not configured")
	}
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 50
	}
	var out struct {
		Code    int                 `json:"code"`
		Message string              `json:"message"`
		Data    AListSearchResponse `json:"data"`
	}
	if err := c.post(ctx, "/api/fs/search", map[string]interface{}{
		"parent":   cleanAListPath(parent),
		"keywords": strings.TrimSpace(keyword),
		"scope":    0,
		"page":     page,
		"per_page": perPage,
		"password": "",
	}, &out); err != nil {
		return nil, err
	}
	if out.Code != http.StatusOK {
		return nil, fmt.Errorf("alist search code=%d message=%s", out.Code, out.Message)
	}
	return &out.Data, nil
}

func (c *AListClient) List(ctx context.Context, parent string, page, perPage int) (*AListListResponse, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("alist client is not configured")
	}
	if page <= 0 {
		page = 1
	}
	if perPage <= 0 {
		perPage = 100
	}
	parent = cleanAListPath(parent)
	var out struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Data    struct {
			Content []struct {
				Name     string    `json:"name"`
				IsDir    bool      `json:"is_dir"`
				Size     int64     `json:"size"`
				Type     int       `json:"type"`
				Modified time.Time `json:"modified"`
			} `json:"content"`
			Total int64 `json:"total"`
		} `json:"data"`
	}
	if err := c.post(ctx, "/api/fs/list", map[string]interface{}{
		"path":     parent,
		"password": "",
		"page":     page,
		"per_page": perPage,
		"refresh":  false,
	}, &out); err != nil {
		return nil, err
	}
	if out.Code != http.StatusOK {
		return nil, fmt.Errorf("alist list code=%d message=%s", out.Code, out.Message)
	}
	items := make([]AListSearchItem, 0, len(out.Data.Content))
	for _, item := range out.Data.Content {
		name := strings.TrimSpace(item.Name)
		if name == "" {
			continue
		}
		items = append(items, AListSearchItem{
			Parent:   parent,
			Name:     name,
			IsDir:    item.IsDir,
			Size:     item.Size,
			Type:     item.Type,
			Modified: item.Modified,
		})
	}
	return &AListListResponse{Content: items, Total: out.Data.Total}, nil
}

func (c *BFFClient) Search(ctx context.Context, parent, keyword string, page, perPage int) (*AListSearchResponse, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("external asset bff is not configured")
	}
	if page > 1 {
		return &AListSearchResponse{}, nil
	}
	if perPage <= 0 {
		perPage = 50
	}
	q := url.Values{}
	q.Set("q", strings.TrimSpace(keyword))
	q.Set("mounts", cleanAListPath(parent))
	q.Set("limit", fmt.Sprintf("%d", perPage))
	q.Set("only_files", "1")
	if len([]rune(strings.TrimSpace(keyword))) >= 2 {
		q.Set("match", "contains")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/search?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return nil, fmt.Errorf("external asset bff search status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	var out struct {
		Items []struct {
			Parent   string    `json:"parent"`
			Name     string    `json:"name"`
			IsDir    bool      `json:"is_dir"`
			Size     int64     `json:"size"`
			Modified time.Time `json:"modified"`
			FullPath string    `json:"full_path"`
		} `json:"items"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	items := make([]AListSearchItem, 0, len(out.Items))
	for _, item := range out.Items {
		parentPath := cleanAListPath(item.Parent)
		name := item.Name
		if strings.TrimSpace(name) == "" && item.FullPath != "" {
			parentPath = cleanAListPath(path.Dir(item.FullPath))
			name = path.Base(item.FullPath)
		}
		if strings.TrimSpace(name) == "" {
			continue
		}
		items = append(items, AListSearchItem{
			Parent:   parentPath,
			Name:     name,
			IsDir:    item.IsDir,
			Size:     item.Size,
			Modified: item.Modified,
		})
	}
	return &AListSearchResponse{Content: items, Total: int64(len(items))}, nil
}

func (c *AListClient) Get(ctx context.Context, filePath string) (*AListFileInfo, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("alist client is not configured")
	}
	var out struct {
		Code    int           `json:"code"`
		Message string        `json:"message"`
		Data    AListFileInfo `json:"data"`
	}
	if err := c.post(ctx, "/api/fs/get", map[string]interface{}{
		"path":     cleanAListPath(filePath),
		"password": "",
	}, &out); err != nil {
		return nil, err
	}
	if out.Code != http.StatusOK {
		return nil, fmt.Errorf("alist get code=%d message=%s", out.Code, out.Message)
	}
	return &out.Data, nil
}

func (c *BFFClient) DirectURL(ctx context.Context, filePath string, inline bool) (string, error) {
	if !c.Enabled() {
		return "", fmt.Errorf("external asset bff is not configured")
	}
	q := url.Values{}
	q.Set("path", cleanAListPath(filePath))
	q.Set("proxy", "0")
	if inline {
		q.Set("inline", "1")
	}
	client := *c.httpClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/fetch?"+q.Encode(), nil)
	if err != nil {
		return "", err
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusSeeOther {
		loc := strings.TrimSpace(resp.Header.Get("Location"))
		if loc == "" {
			return "", fmt.Errorf("external asset bff returned redirect without location")
		}
		return loc, nil
	}
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return "", fmt.Errorf("external asset bff direct link unavailable: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
}

func (c *BFFClient) FetchAvailable(ctx context.Context, filePath string) (bool, error) {
	if !c.Enabled() {
		return false, fmt.Errorf("external asset bff is not configured")
	}
	q := url.Values{}
	q.Set("path", cleanAListPath(filePath))
	q.Set("proxy", "0")
	client := *c.httpClient
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/fetch?"+q.Encode(), nil)
	if err != nil {
		return false, err
	}
	req.Header.Set("Range", "bytes=0-0")
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	switch {
	case resp.StatusCode >= 200 && resp.StatusCode < 300:
		return true, nil
	case resp.StatusCode == http.StatusFound || resp.StatusCode == http.StatusTemporaryRedirect || resp.StatusCode == http.StatusSeeOther || resp.StatusCode == http.StatusPermanentRedirect:
		return strings.TrimSpace(resp.Header.Get("Location")) != "", nil
	case resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone:
		return false, nil
	default:
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		return false, fmt.Errorf("external asset bff fetch probe status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
}

func (c *BFFClient) BrowserFetchURL(filePath string, inline bool, proxy bool) string {
	if !c.Enabled() || strings.TrimSpace(c.browserBaseURL) == "" {
		return ""
	}
	q := url.Values{}
	q.Set("path", cleanAListPath(filePath))
	if proxy {
		q.Set("proxy", "1")
	} else {
		q.Set("proxy", "0")
	}
	if inline {
		q.Set("inline", "1")
	}
	return c.browserBaseURL + "/api/fetch?" + q.Encode()
}

func (c *BFFClient) OpenFetch(ctx context.Context, filePath string, inline bool) (io.ReadCloser, error) {
	if !c.Enabled() {
		return nil, fmt.Errorf("external asset bff is not configured")
	}
	q := url.Values{}
	q.Set("path", cleanAListPath(filePath))
	q.Set("proxy", "1")
	if inline {
		q.Set("inline", "1")
	}
	client := *c.httpClient
	client.Timeout = 0
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/api/fetch?"+q.Encode(), nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("external asset bff fetch status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	return resp.Body, nil
}

func (c *AListClient) OpenRawURL(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	return openHTTPBody(ctx, rawURL)
}

func openHTTPBody(ctx context.Context, rawURL string) (io.ReadCloser, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return nil, fmt.Errorf("raw url is required")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := (&http.Client{Timeout: 30 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
		_ = resp.Body.Close()
		return nil, fmt.Errorf("open raw url status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(body)))
	}
	return resp.Body, nil
}

func (c *AListClient) post(ctx context.Context, apiPath string, payload interface{}, target interface{}) error {
	body, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+apiPath, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", c.token)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("alist %s status=%d body=%s", apiPath, resp.StatusCode, strings.TrimSpace(string(raw)))
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		return err
	}
	return nil
}

func (c *AListClient) IsAListURL(raw string) bool {
	if c == nil {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	base, err := url.Parse(c.baseURL)
	if err != nil || base.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, base.Host)
}

func (c *BFFClient) IsBFFURL(raw string) bool {
	if c == nil {
		return false
	}
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u.Host == "" {
		return false
	}
	base, err := url.Parse(c.baseURL)
	if err != nil || base.Host == "" {
		return false
	}
	return strings.EqualFold(u.Host, base.Host)
}

func joinAListPath(parent, name string) string {
	parent = cleanAListPath(parent)
	name = strings.ReplaceAll(name, "\\", "/")
	if strings.TrimSpace(name) == "" {
		return parent
	}
	return path.Join(parent, name)
}

func cleanAListPath(value string) string {
	value = strings.TrimSpace(strings.ReplaceAll(value, "\\", "/"))
	if value == "" {
		return "/"
	}
	return path.Clean("/" + strings.TrimLeft(value, "/"))
}
