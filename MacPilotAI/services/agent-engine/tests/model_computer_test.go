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
	"github.com/macpilot/agent-engine/internal/vm"
	"github.com/macpilot/agent-engine/pkg/protocol"
)

func TestModelPredictsComputerActions(t *testing.T) {
	provider := providers.NewMetaMuseProvider("", "muse-wonder-1.0", "")
	ctx := context.Background()

	testCases := []struct {
		goal           string
		expectedAction protocol.ActionType
	}{
		{"Hey Jarvis, start computer", protocol.ActionComputerStart},
		{"Boot virtual machine", protocol.ActionComputerStart},
		{"Check computer status", protocol.ActionComputerStatus},
		{"Show computer metrics", protocol.ActionComputerGetMetrics},
		{"Take computer screenshot", protocol.ActionComputerScreenshot},
		{"Run in computer uname -a", protocol.ActionComputerExecute},
		{"Execute in computer ls -la /home", protocol.ActionComputerExecute},
		{"Stop computer safely", protocol.ActionComputerStop},
	}

	for _, tc := range testCases {
		t.Run(tc.goal, func(t *testing.T) {
			action, err := provider.PredictAction(ctx, tc.goal, nil)
			if err != nil {
				t.Fatalf("Model prediction failed for '%s': %v", tc.goal, err)
			}
			if action.Action != tc.expectedAction {
				t.Errorf("For goal '%s': expected action %s, got %s", tc.goal, tc.expectedAction, action.Action)
			}
			if action.Speech == "" {
				t.Errorf("Expected natural speech response for '%s', got empty", tc.goal)
			}
			t.Logf("✅ Goal: %-35s -> Action: %-25s | Speech: %s", tc.goal, action.Action, action.Speech)
		})
	}
}

func TestOrchestratorComputerTaskExecution(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "macpilot_orch_test_*")
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
	vmManager := vm.NewComputerManager(db, filepath.Join(tempDir, "vm"))
	orch := orchestrator.NewOrchestrator(db, provider, apprManager, vmManager)

	eventsReceived := make(chan protocol.TaskEvent, 10)
	orch.Subscribe(func(event protocol.TaskEvent) {
		eventsReceived <- event
	})

	ctx := context.Background()
	taskID, err := orch.SubmitTask(ctx, protocol.TaskRequest{
		Goal:     "Check computer status",
		Model:    "muse-wonder-1.0",
		MaxSteps: 3,
	})
	if err != nil {
		t.Fatalf("Failed to submit task: %v", err)
	}

	var completed bool
	timeout := time.After(3 * time.Second)

	for !completed {
		select {
		case ev := <-eventsReceived:
			t.Logf("Event: type=%s step=%d message=%s speech=%s", ev.Type, ev.Step, ev.Message, ev.Speech)
			if ev.Type == protocol.EventTaskCompleted {
				completed = true
				if !ev.Success {
					t.Errorf("Expected completed event to have success=true")
				}
			}
		case <-timeout:
			t.Fatalf("Task %s did not complete within timeout", taskID)
		}
	}
	t.Logf("✅ End-to-end computer status orchestration verified successfully.")
}

