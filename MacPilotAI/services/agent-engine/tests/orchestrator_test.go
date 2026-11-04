package tests

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/macpilot/agent-engine/internal/approvals"
	"github.com/macpilot/agent-engine/internal/orchestrator"
	"github.com/macpilot/agent-engine/internal/persistence"
	"github.com/macpilot/agent-engine/internal/providers"
	"github.com/macpilot/agent-engine/pkg/protocol"
)

func TestOrchestratorTaskExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "macpilot_test_*")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	dbPath := filepath.Join(tempDir, "test.db")
	db, err := persistence.NewDB(dbPath)
	if err != nil {
		t.Fatalf("Failed to initialize test DB: %v", err)
	}
	defer db.Close()

	apprManager := approvals.NewManager(db)
	provider := providers.NewMetaMuseProvider("", "muse-wonder-1.0", "")
	orch := orchestrator.NewOrchestrator(db, provider, apprManager, nil)

	completedChan := make(chan protocol.TaskEvent, 1)
	orch.Subscribe(func(event protocol.TaskEvent) {
		if event.Type == protocol.EventTaskCompleted {
			completedChan <- event
		}
	})

	ctx := context.Background()
	taskID, err := orch.SubmitTask(ctx, protocol.TaskRequest{
		Goal:     "open chrome and play weekend song",
		Model:    "muse-wonder-1.0",
		MaxSteps: 5,
	})
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}
	if taskID == "" {
		t.Errorf("Expected non-empty task ID")
	}

	select {
	case event := <-completedChan:
		if !event.Success {
			t.Errorf("Expected task to complete with success=true, got false")
		}
	case <-time.After(5 * time.Second):
		t.Errorf("Task did not complete within 5 seconds")
	}
}

