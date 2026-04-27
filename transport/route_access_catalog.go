package transport

import (
	"sort"
	"strings"
	"sync"

	"workflow/domain"
)

type RouteAccessCatalog struct {
	mu    sync.RWMutex
	rules []domain.RouteAccessRule
}

func NewRouteAccessCatalog() *RouteAccessCatalog {
	return &RouteAccessCatalog{}
}

func (c *RouteAccessCatalog) Reset() {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules = c.rules[:0]
}

func (c *RouteAccessCatalog) AddRule(rule domain.RouteAccessRule) {
	if c == nil {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.rules = append(c.rules, rule)
}

func (c *RouteAccessCatalog) ListRouteAccessRules() []domain.RouteAccessRule {
	if c == nil {
		return nil
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	out := make([]domain.RouteAccessRule, len(c.rules))
	copy(out, c.rules)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})
	return out
}

func (c *RouteAccessCatalog) FindRouteAccessRule(method, path string) (domain.RouteAccessRule, bool) {
	if c == nil {
		return domain.RouteAccessRule{}, false
	}
	c.mu.RLock()
	defer c.mu.RUnlock()

	method = strings.TrimSpace(method)
	path = strings.TrimSpace(path)
	for _, rule := range c.rules {
		if rule.Method == method && rule.Path == path {
			return rule, true
		}
	}
	return domain.RouteAccessRule{}, false
}

func joinRoutePath(basePath, relativePath string) string {
	basePath = strings.TrimRight(strings.TrimSpace(basePath), "/")
	relativePath = strings.TrimSpace(relativePath)
	if relativePath == "" {
		if basePath == "" {
			return "/"
		}
		return basePath
	}
	if !strings.HasPrefix(relativePath, "/") {
		relativePath = "/" + relativePath
	}
	if basePath == "" || basePath == "/" {
		return relativePath
	}
	return basePath + relativePath
}
