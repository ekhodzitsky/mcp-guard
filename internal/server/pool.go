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
	config    map[string]config.ServerConfig
	processes map[string]*Process
	checkers  map[string]*HealthChecker
	bus       *events.Bus
	started   bool
	cancel    context.CancelFunc
	restartMu sync.Mutex
}

// NewPool creates a new process pool.
func NewPool(configs map[string]config.ServerConfig, bus *events.Bus) *Pool {
	cpy := make(map[string]config.ServerConfig, len(configs))
	for k, v := range configs {
		cpy[k] = v
	}
	return &Pool{
		config:    cpy,
		processes: make(map[string]*Process),
		checkers:  make(map[string]*HealthChecker),
		bus:       bus,
	}
}

// Start launches all configured servers and their health checkers.
func (p *Pool) Start(ctx context.Context) error {
	p.mu.Lock()
	if p.started {
		p.mu.Unlock()
		return fmt.Errorf("pool already started")
	}
	p.started = true
	startCtx, cancel := context.WithCancel(ctx)
	p.cancel = cancel
	p.mu.Unlock()

	var started []string
	for name, cfg := range p.config {
		proc := NewProcess(name, cfg, p.bus)
		if err := proc.Start(ctx); err != nil {
			p.cleanupOnStartFailure(started)
			return fmt.Errorf("start server %q: %w", name, err)
		}

		checker := NewHealthChecker(proc, p.bus, 5*time.Second, 3)
		checker.Start(ctx)

		p.mu.Lock()
		p.processes[name] = proc
		p.checkers[name] = checker
		p.mu.Unlock()
		started = append(started, name)

		if p.bus != nil {
			go p.serverRestarter(startCtx, name)
		}
	}

	return nil
}

func (p *Pool) cleanupOnStartFailure(started []string) {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	processes := make(map[string]*Process, len(started))
	checkers := make(map[string]*HealthChecker, len(started))
	for _, name := range started {
		processes[name] = p.processes[name]
		checkers[name] = p.checkers[name]
	}
	p.processes = make(map[string]*Process)
	p.checkers = make(map[string]*HealthChecker)
	p.started = false
	p.cancel = nil
	p.mu.Unlock()

	for _, c := range checkers {
		c.Stop()
	}
	var wg sync.WaitGroup
	for _, proc := range processes {
		wg.Add(1)
		go func(pr *Process) {
			defer wg.Done()
			stopCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = pr.Stop(stopCtx)
		}(proc)
	}
	wg.Wait()
}

// Stop gracefully stops all processes.
func (p *Pool) Stop(ctx context.Context) error {
	p.mu.Lock()
	if p.cancel != nil {
		p.cancel()
	}
	checkers := make(map[string]*HealthChecker, len(p.checkers))
	for k, v := range p.checkers {
		checkers[k] = v
	}
	processes := make(map[string]*Process, len(p.processes))
	for k, v := range p.processes {
		processes[k] = v
	}
	p.processes = make(map[string]*Process)
	p.checkers = make(map[string]*HealthChecker)
	p.started = false
	p.cancel = nil
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

// Names returns all currently running server names.
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
	p.restartMu.Lock()
	defer p.restartMu.Unlock()

	p.mu.RLock()
	proc := p.processes[name]
	checker := p.checkers[name]
	cfg := p.config[name]
	p.mu.RUnlock()

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
		p.mu.Lock()
		delete(p.processes, name)
		delete(p.checkers, name)
		p.mu.Unlock()
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

func (p *Pool) serverRestarter(ctx context.Context, name string) {
	if p.bus == nil {
		return
	}
	ch := p.bus.Subscribe(name)
	defer p.bus.Unsubscribe(name, ch)

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
			checker := p.checkers[name]
			p.mu.RUnlock()
			if checker == nil {
				continue
			}
			if checker.Failures() == 3 {
				go p.attemptRestart(ctx, name)
			}
		case <-ctx.Done():
			return
		}
	}
}

func (p *Pool) attemptRestart(ctx context.Context, name string) {
	cfg := p.config[name]
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
