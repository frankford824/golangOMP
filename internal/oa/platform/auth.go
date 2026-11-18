// Package platform shares the OA server's request identity and response envelope.
package platform

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"io"
	"maps"
	"net/http"
	"strings"
	"sync"
	"time"
)

type Actor struct {
	Activated         bool            `json:"activated"`
	ID                int64           `json:"id"`
	EmployeeNo        *int            `json:"employee_no"`
	Name              string          `json:"name"`
	DepartmentID      int64           `json:"department_id"`
	TeamID            int64           `json:"team_id"`
	Department        string          `json:"department"`
	Team              string          `json:"team"`
	SuperAdmin        bool            `json:"super_admin"`
	Actions           map[string]bool `json:"actions"`
	Roles             []string        `json:"roles"`
	PeopleDepartments []int64         `json:"people_departments,omitempty"`
	PeopleTeams       []int64         `json:"people_teams,omitempty"`
}

type actorKey struct{}

func ActorFrom(r *http.Request) Actor {
	a, _ := r.Context().Value(actorKey{}).(Actor)
	return a
}

func WithActor(r *http.Request, actor Actor) *http.Request {
	return r.WithContext(context.WithValue(r.Context(), actorKey{}, actor))
}

func JSON(w http.ResponseWriter, status int, data any) {
	write(w, status, map[string]any{"data": data})
}

func Error(w http.ResponseWriter, status int, message string) {
	write(w, status, map[string]string{"error": message})
}

func write(w http.ResponseWriter, status int, body any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(body)
}

type Identity struct {
	Base   string
	Client *http.Client
}

func NewIdentity(base string) Identity {
	return Identity{Base: strings.TrimRight(base, "/"), Client: &http.Client{
		Timeout:       5 * time.Second,
		CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse },
	}}
}

// Authenticate coalesces simultaneous checks of the same bearer without caching
// completed authorizations. Each business request gets its own mutable capability map.
func (i Identity) Authenticate(next http.Handler) http.Handler {
	type pending struct {
		done   chan struct{}
		actor  Actor
		status int
	}
	var mu sync.Mutex
	flights := make(map[[32]byte]*pending)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		auth := strings.Fields(r.Header.Get("Authorization"))
		if len(auth) != 2 || !strings.EqualFold(auth[0], "Bearer") || len(auth[1]) > 8192 {
			Error(w, 401, "请登录运营平台")
			return
		}
		key := sha256.Sum256([]byte(auth[1]))
		mu.Lock()
		f := flights[key]
		if f == nil {
			f = &pending{done: make(chan struct{})}
			flights[key] = f
			go func(call *pending, token string) {
				ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()
				call.actor, call.status = i.resolve(ctx, token)
				mu.Lock()
				delete(flights, key)
				close(call.done)
				mu.Unlock()
			}(f, auth[1])
		}
		mu.Unlock()
		select {
		case <-r.Context().Done():
			return
		case <-f.done:
		}
		if f.status != 200 {
			msg := "账号服务暂时不可用"
			if f.status == 401 || f.status == 403 {
				msg = "登录已失效或账号未启用"
			}
			Error(w, f.status, msg)
			return
		}
		actor := f.actor
		actor.Actions = maps.Clone(actor.Actions)
		next.ServeHTTP(w, WithActor(r, actor))
	})
}

func (i Identity) resolve(ctx context.Context, token string) (Actor, int) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, i.Base+"/v1/auth/me", nil)
	if err != nil {
		return Actor{}, 503
	}
	req.Header.Set("Authorization", "Bearer "+token)
	resp, err := i.Client.Do(req)
	if err != nil {
		return Actor{}, 503
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 65536))
		if resp.StatusCode == 401 || resp.StatusCode == 403 {
			return Actor{}, resp.StatusCode
		}
		return Actor{}, 503
	}
	actor, err := decodeActor(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return Actor{}, 503
	}
	if actor.ID <= 0 {
		return Actor{}, 403
	}
	return actor, 200
}

func decodeActor(reader io.Reader) (Actor, error) {
	var result struct {
		Data struct {
			ID           int64  `json:"id"`
			EmployeeNo   *int   `json:"employee_no"`
			Name         string `json:"display_name"`
			Status       string `json:"status"`
			Department   string `json:"department"`
			Team         string `json:"team"`
			DepartmentID int64  `json:"department_id"`
			TeamID       int64  `json:"team_id"`
			Access       struct {
				SuperAdmin bool     `json:"is_super_admin"`
				Actions    []string `json:"actions"`
				Roles      []string `json:"roles"`
			} `json:"frontend_access"`
		} `json:"data"`
	}
	if err := json.NewDecoder(reader).Decode(&result); err != nil {
		return Actor{}, err
	}
	u := result.Data
	if u.ID <= 0 {
		return Actor{}, errors.New("missing authenticated identity")
	}
	if u.Status != "active" {
		return Actor{}, nil
	}
	a := Actor{ID: u.ID, EmployeeNo: u.EmployeeNo, Name: u.Name, Department: u.Department, Team: u.Team, DepartmentID: u.DepartmentID, TeamID: u.TeamID, SuperAdmin: u.Access.SuperAdmin, Roles: u.Access.Roles, Actions: map[string]bool{}}
	for _, permission := range u.Access.Actions {
		a.Actions[permission] = true
	}
	return a, nil
}
