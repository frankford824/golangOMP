// Package scheduler provides shared cron infrastructure for cleanup jobs.
//
// TODO(R6.D): rename package from service/asset_lifecycle/scheduler to a
// neutral location once SA-A v2.1 docs are revised.
package scheduler

import (
	"context"
	"fmt"
	"log"
	"sync"

	"github.com/robfig/cron/v3"
)

// JobFunc is the unit a Cron entry executes. It receives a context that is
// cancelled when the cron is stopped or the parent context is cancelled.
type JobFunc func(ctx context.Context) error

// Cron wraps robfig/cron/v3 with structured logging and a cancellable context.
type Cron struct {
	inner  *cron.Cron
	parent context.Context
	cancel context.CancelFunc
	logger *log.Logger
	mu     sync.Mutex
	jobs   []entry
}

type entry struct {
	name string
	spec string
	id   cron.EntryID
}

// EntryInfo is a read-only snapshot of a registered cron entry.
type EntryInfo struct {
	Name string
	Spec string
}

// Config is retained for compatibility with the pre-R6 dummy scheduler tests.
// New cron wiring should use Cron directly.
type Config struct {
	Enabled bool
}

// DefaultConfig returns the disabled legacy config.
func DefaultConfig() Config {
	return Config{Enabled: false}
}

// Register retains the pre-R6 dummy scheduler behavior for legacy callers.
// New cron wiring should use New/Add/Start/Stop.
func Register(ctx context.Context, cfg Config, job JobFunc) error {
	if !cfg.Enabled || job == nil {
		return nil
	}
	return job(ctx)
}

// New creates a stopped Cron. Caller must call Add(...) then Start().
func New(parent context.Context, logger *log.Logger) *Cron {
	if parent == nil {
		parent = context.Background()
	}
	if logger == nil {
		logger = log.Default()
	}
	ctx, cancel := context.WithCancel(parent)
	return &Cron{
		inner:  cron.New(),
		parent: ctx,
		cancel: cancel,
		logger: logger,
	}
}

// Add registers a named job with the given cron spec. Returns error if the
// spec fails to parse. Safe to call before Start. After Start, additional Add
// calls schedule immediately.
func (c *Cron) Add(name, spec string, job JobFunc) error {
	if c == nil {
		return fmt.Errorf("cron is nil")
	}
	if job == nil {
		return fmt.Errorf("cron job %q is nil", name)
	}
	id, err := c.inner.AddFunc(spec, c.wrap(name, job))
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.jobs = append(c.jobs, entry{name: name, spec: spec, id: id})
	c.mu.Unlock()
	return nil
}

// Start begins ticking. Idempotent.
func (c *Cron) Start() {
	if c == nil {
		return
	}
	c.inner.Start()
}

// Stop signals all running jobs to finish and waits up to ctx timeout. Returns
// nil when cron stops cleanly, or ctx.Err() when the provided context wins.
func (c *Cron) Stop(ctx context.Context) error {
	if c == nil {
		return nil
	}
	c.cancel()
	done := c.inner.Stop()
	select {
	case <-done.Done():
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Entries returns a snapshot of currently registered job names and specs. Used
// by tests and observability hooks.
func (c *Cron) Entries() []EntryInfo {
	if c == nil {
		return nil
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	out := make([]EntryInfo, 0, len(c.jobs))
	for _, job := range c.jobs {
		out = append(out, EntryInfo{Name: job.name, Spec: job.spec})
	}
	return out
}

func (c *Cron) wrap(name string, job JobFunc) func() {
	return func() {
		defer func() {
			if recovered := recover(); recovered != nil {
				c.logger.Printf("[CRON] job=%s panic=%v", name, recovered)
			}
		}()
		if err := job(c.parent); err != nil {
			c.logger.Printf("[CRON] job=%s err=%v", name, err)
		}
	}
}
