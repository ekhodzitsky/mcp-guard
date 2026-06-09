package server

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
)

// Pool manages a collection of MCP server processes.
type Pool struct {
	mu        sync.RWMutex
	configs   map[string]config.ServerConfig
	processes map[string]*Process
	checkers  map[string]*HealthChecker
	bus       *events.Bus
}

// NewPool creates a new process pool.
func NewPool(configs map[string]config.ServerConfig, bus *events.Bus) *Pool {
	return &Pool{
		configs:   configs,
		processes: make(map[string]*Process),
		checkers:  make(map[string]*HealthChecker),
		bus:       bus,
	}
}

// Start launches all configured servers and their health checkers.
func (p *Pool) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	for name, cfg := range p.configs {
		proc := NewProcess(name, cfg, p.bus)
		if err := proc.Start(ctx); err != nil {
			return fmt.Errorf("start server %q: %w", name, err)
		}
		p.processes[name] = proc

		checker := NewHealthChecker(proc, p.bus, 5*time.Second, 3)
		checker.Start(ctx)
		p.checkers[name] = checker
	}

	go p.restarter(ctx)

	return nil
}

// Stop gracefully stops all processes.
func (p *Pool) Stop(ctx context.Context) error {
	p.mu.Lock()
	checkers := make(map[string]*HealthChecker, len(p.checkers))
	for k, v := range p.checkers {
		checkers[k] = v
	}
	processes := make(map[string]*Process, len(p.processes))
	for k, v := range p.processes {
		processes[k] = v
	}
	p.mu.Unlock()

	for _, c := range checkers {
		c.Stop()
	}

	var wg sync.WaitGroup
	errCh := make(chan error, len(processes))
	for _, proc := range processes {
		wg.Add(1)
		go func(pr *Process) {
			defer wg.Done()
			if err := pr.Stop(ctx); err != nil {
				errCh <- err
			}
		}(proc)
	}
	wg.Wait()
	close(errCh)

	var errs []error
	for err := range errCh {
		errs = append(errs, err)
	}
	if len(errs) > 0 {
		return fmt.Errorf("stop pool: %v", errs)
	}
	return nil
}

// Get returns a process by name.
func (p *Pool) Get(name string) *Process {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.processes[name]
}

// Names returns all configured server names.
func (p *Pool) Names() []string {
	p.mu.RLock()
	defer p.mu.RUnlock()
	names := make([]string, 0, len(p.processes))
	for n := range p.processes {
		names = append(names, n)
	}
	return names
}

// Restart restarts a single server with exponential backoff.
func (p *Pool) Restart(ctx context.Context, name string) error {
	p.mu.Lock()
	proc := p.processes[name]
	checker := p.checkers[name]
	cfg := p.configs[name]
	p.mu.Unlock()

	if checker != nil {
		checker.Stop()
	}
	if proc != nil {
		stopCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		defer cancel()
		_ = proc.Stop(stopCtx)
	}

	newProc := NewProcess(name, cfg, p.bus)
	if err := newProc.Start(ctx); err != nil {
		return fmt.Errorf("restart server %q: %w", name, err)
	}

	p.mu.Lock()
	p.processes[name] = newProc
	newChecker := NewHealthChecker(newProc, p.bus, 5*time.Second, 3)
	newChecker.Start(ctx)
	p.checkers[name] = newChecker
	p.mu.Unlock()

	return nil
}

func (p *Pool) restarter(ctx context.Context) {
	if p.bus == nil {
		return
	}
	ch := p.bus.Subscribe("")
	defer p.bus.Unsubscribe("", ch)

	for {
		select {
		case evt, ok := <-ch:
			if !ok {
				return
			}
			if evt.Type != "health.failed" {
				continue
			}
			p.mu.RLock()
			checker := p.checkers[evt.Server]
			p.mu.RUnlock()
			if checker == nil {
				continue
			}
			if checker.Failures() >= 3 {
				go p.attemptRestart(ctx, evt.Server)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pool) attemptRestart(ctx context.Context, name string) {
	cfg := p.configs[name]
	backoff := time.Second
	for i := 0; i < cfg.Restart.MaxAttempts; i++ {
		if err := p.Restart(ctx, name); err == nil {
			return
		}
		select {
		case <-time.After(backoff):
			backoff *= 2
			if backoff > 30*time.Second {
				backoff = 30 * time.Second
			}
		case <-ctx.Done():
			return
		}
	}
	if p.bus != nil {
		p.bus.Publish(ctx, events.Event{
			Type:    "process.failed",
			Server:  name,
			Payload: map[string]string{"reason": "max restart attempts exceeded"},
		})
	}
}
