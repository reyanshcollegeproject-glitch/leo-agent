package approvals

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/macpilot/agent-engine/pkg/protocol"
	"github.com/macpilot/agent-engine/internal/persistence"
)

type Manager struct {
	db       *persistence.DB
	pending  map[string]chan bool
	requests map[string]*protocol.ApprovalRequest
	mu       sync.RWMutex
}

func NewManager(db *persistence.DB) *Manager {
	return &Manager{
		db:       db,
		pending:  make(map[string]chan bool),
		requests: make(map[string]*protocol.ApprovalRequest),
	}
}

func (m *Manager) RequestApproval(ctx context.Context, taskID, command, reason, severity string) (bool, error) {
	id := uuid.New().String()
	ch := make(chan bool, 1)

	req := &protocol.ApprovalRequest{
		ID:        id,
		TaskID:    taskID,
		Command:   command,
		Reason:    reason,
		Severity:  severity,
		CreatedAt: time.Now().UTC(),
	}

	m.mu.Lock()
	m.pending[id] = ch
	m.requests[id] = req
	m.mu.Unlock()

	if err := m.db.CreateApproval(id, taskID, command, reason, severity); err != nil {
		return false, fmt.Errorf("failed to persist approval request: %w", err)
	}

	select {
	case approved := <-ch:
		m.mu.Lock()
		delete(m.pending, id)
		delete(m.requests, id)
		m.mu.Unlock()
		_ = m.db.ResolveApproval(id, approved)
		return approved, nil

	case <-ctx.Done():
		m.mu.Lock()
		delete(m.pending, id)
		delete(m.requests, id)
		m.mu.Unlock()
		_ = m.db.ResolveApproval(id, false)
		return false, ctx.Err()

	case <-time.After(5 * time.Minute): // 5 minute approval timeout
		m.mu.Lock()
		delete(m.pending, id)
		delete(m.requests, id)
		m.mu.Unlock()
		_ = m.db.ResolveApproval(id, false)
		return false, fmt.Errorf("approval request timed out after 5 minutes")
	}
}

func (m *Manager) Resolve(id string, approved bool) error {
	m.mu.RLock()
	ch, exists := m.pending[id]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("no pending approval found for ID: %s", id)
	}

	ch <- approved
	return nil
}

func (m *Manager) ListPending() []*protocol.ApprovalRequest {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var list []*protocol.ApprovalRequest
	for _, req := range m.requests {
		list = append(list, req)
	}
	return list
}
