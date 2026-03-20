package cli

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

type callRecord struct {
	name string
	args []string
}

type mockResponse struct {
	output []byte
	err    error
	delay  time.Duration
}

// MockExecutor is a test double for CommandRunner.
type MockExecutor struct {
	mu        sync.Mutex
	responses map[string]mockResponse
	calls     []callRecord
}

func NewMockExecutor() *MockExecutor {
	return &MockExecutor{responses: make(map[string]mockResponse)}
}

func key(name string, args []string) string {
	return name + " " + strings.Join(args, " ")
}

func (m *MockExecutor) SetResponse(name string, args []string, output []byte, err error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.responses[key(name, args)] = mockResponse{output: output, err: err}
}

func (m *MockExecutor) SetDelay(name string, args []string, d time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	k := key(name, args)
	r := m.responses[k]
	r.delay = d
	m.responses[k] = r
}

func (m *MockExecutor) Run(ctx context.Context, name string, args []string) ([]byte, error) {
	m.mu.Lock()
	m.calls = append(m.calls, callRecord{name, args})
	r, ok := m.responses[key(name, args)]
	m.mu.Unlock()

	if !ok {
		return nil, fmt.Errorf("%s: %w", name, ErrNotInstalled)
	}
	if r.delay > 0 {
		select {
		case <-time.After(r.delay):
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	return r.output, r.err
}

func (m *MockExecutor) CallsFor(name string) []callRecord {
	m.mu.Lock()
	defer m.mu.Unlock()
	var out []callRecord
	for _, c := range m.calls {
		if c.name == name {
			out = append(out, c)
		}
	}
	return out
}
