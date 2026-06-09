// Package server manages MCP server processes.
package server

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"

	"github.com/ekhodzitsky/mcp-guard/internal/config"
	"github.com/ekhodzitsky/mcp-guard/internal/events"
)

// Process represents a single MCP server process.
type Process struct {
	name      string
	cfg       config.ServerConfig
	bus       *events.Bus
	cmd       *exec.Cmd
	stdin     io.WriteCloser
	stdout    io.ReadCloser
	scanner   *bufio.Scanner
	responses chan []byte
	mu        sync.RWMutex
	running   bool
}

// NewProcess creates a new process handle.
func NewProcess(name string, cfg config.ServerConfig, bus *events.Bus) *Process {
	return &Process{
		name: name,
		cfg:  cfg,
		bus:  bus,
	}
}

// Start launches the server process.
// The provided ctx must remain valid for the lifetime of the process;
// cancelling it prematurely will kill the process.
func (p *Process) Start(ctx context.Context) error {
	p.mu.Lock()
	defer p.mu.Unlock()

	if p.running {
		return fmt.Errorf("process %q already running", p.name)
	}

	cmd := exec.CommandContext(ctx, p.cfg.Command, p.cfg.Args...)
	cmd.Env = os.Environ()
	for k, v := range p.cfg.Env {
		cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, v))
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		_ = stdin.Close()
		return fmt.Errorf("stdout pipe: %w", err)
	}
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		_ = stdin.Close()
		_ = stdout.Close()
		return fmt.Errorf("start command: %w", err)
	}

	p.cmd = cmd
	p.stdin = stdin
	p.stdout = stdout
	p.scanner = bufio.NewScanner(stdout)
	p.scanner.Buffer(make([]byte, 4096), 10*1024*1024)
	p.responses = make(chan []byte, 64)
	go p.readLoop()
	p.running = true

	if p.bus != nil {
		p.bus.Publish(ctx, events.Event{
			Type:   "process.started",
			Server: p.name,
		})
	}

	return nil
}

// readLoop reads lines from the scanner and pushes them to the responses channel.
// It exits when the scanner returns EOF or error, closing the channel.
func (p *Process) readLoop() {
	scanner := p.scanner
	for scanner.Scan() {
		line := append([]byte(nil), scanner.Bytes()...)
		p.mu.RLock()
		ch := p.responses
		p.mu.RUnlock()
		if ch == nil {
			return
		}
		ch <- line
	}
	p.mu.RLock()
	ch := p.responses
	p.mu.RUnlock()
	if ch != nil {
		close(ch)
	}
}

// Stop gracefully stops the process.
func (p *Process) Stop(ctx context.Context) error {
	p.mu.Lock()
	cmd := p.cmd
	running := p.running
	stdin := p.stdin
	stdout := p.stdout
	p.mu.Unlock()

	if !running || cmd == nil {
		return nil
	}

	if cmd.Process != nil {
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGTERM)
	}

	done := make(chan error, 1)
	go func() {
		done <- cmd.Wait()
	}()

	select {
	case err := <-done:
		p.cleanupAfterStop(stdin, stdout)

		if p.bus != nil {
			p.bus.Publish(ctx, events.Event{
				Type:   "process.stopped",
				Server: p.name,
			})
		}

		// Intentional signal termination is not an error.
		if exitErr, ok := err.(*exec.ExitError); ok {
			if status, ok := exitErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
				return nil
			}
		}
		return err
	case <-ctx.Done():
		if cmd.Process != nil {
			_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		}
		<-done

		p.cleanupAfterStop(stdin, stdout)

		if p.bus != nil {
			p.bus.Publish(ctx, events.Event{
				Type:   "process.stopped",
				Server: p.name,
			})
		}

		return ctx.Err()
	}
}

func (p *Process) cleanupAfterStop(stdin io.WriteCloser, stdout io.ReadCloser) {
	p.mu.Lock()
	p.running = false
	p.cmd = nil
	if stdin != nil {
		_ = stdin.Close()
	}
	if stdout != nil {
		_ = stdout.Close()
	}
	p.stdin = nil
	p.stdout = nil
	p.scanner = nil
	p.responses = nil
	p.mu.Unlock()
}

// Running reports whether the process is active.
func (p *Process) Running() bool {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.running
}

// Stdin returns the process stdin.
func (p *Process) Stdin() io.WriteCloser {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stdin
}

// Stdout returns the process stdout.
func (p *Process) Stdout() io.ReadCloser {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.stdout
}

// Scanner returns the process stdout scanner.
func (p *Process) Scanner() *bufio.Scanner {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.scanner
}

// Responses returns the channel of response lines from the process.
func (p *Process) Responses() <-chan []byte {
	p.mu.RLock()
	defer p.mu.RUnlock()
	return p.responses
}

// Name returns the process name.
func (p *Process) Name() string {
	return p.name
}
