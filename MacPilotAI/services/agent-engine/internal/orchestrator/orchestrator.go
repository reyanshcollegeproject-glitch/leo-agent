package orchestrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/macpilot/agent-engine/pkg/protocol"
	"github.com/macpilot/agent-engine/internal/approvals"
	"github.com/macpilot/agent-engine/internal/computer"
	"github.com/macpilot/agent-engine/internal/persistence"
	"github.com/macpilot/agent-engine/internal/policy"
	"github.com/macpilot/agent-engine/internal/providers"
	"github.com/macpilot/agent-engine/internal/verification"
	"github.com/macpilot/agent-engine/internal/vm"
)

type EventListener func(event protocol.TaskEvent)

type Orchestrator struct {
	db          *persistence.DB
	policy      *policy.SentinelPolicy
	approvals   *approvals.Manager
	controller  *computer.Controller
	verifier    *verification.Verifier
	provider    providers.Provider
	vmManager   *vm.ComputerManager
	tasks       map[string]context.CancelFunc
	listeners   []EventListener
	listenersMu sync.RWMutex
	tasksMu     sync.Mutex
}

func NewOrchestrator(
	db *persistence.DB,
	provider providers.Provider,
	apprManager *approvals.Manager,
	vmManager *vm.ComputerManager,
) *Orchestrator {
	return &Orchestrator{
		db:         db,
		policy:     policy.NewSentinelPolicy(),
		approvals:  apprManager,
		controller: computer.NewController(),
		verifier:   verification.NewVerifier(),
		provider:   provider,
		vmManager:  vmManager,
		tasks:      make(map[string]context.CancelFunc),
	}
}

func (o *Orchestrator) Subscribe(listener EventListener) {
	o.listenersMu.Lock()
	defer o.listenersMu.Unlock()
	o.listeners = append(o.listeners, listener)
}

func (o *Orchestrator) emitEvent(event protocol.TaskEvent) {
	event.Timestamp = time.Now().UTC()
	payload, _ := json.Marshal(event)
	_ = o.db.RecordEvent(event.TaskID, string(event.Type), string(payload))

	o.listenersMu.RLock()
	defer o.listenersMu.RUnlock()
	for _, l := range o.listeners {
		go l(event)
	}
}

func (o *Orchestrator) SubmitTask(ctx context.Context, req protocol.TaskRequest) (string, error) {
	taskID := req.ID
	if taskID == "" {
		taskID = uuid.New().String()
	}

	maxSteps := req.MaxSteps
	if maxSteps <= 0 {
		maxSteps = 25
	}

	model := req.Model
	if model == "" {
		model = "muse-wonder-1.0"
	}

	if err := o.db.CreateTask(taskID, req.Goal, model, maxSteps); err != nil {
		return "", fmt.Errorf("failed to create task: %w", err)
	}

	taskCtx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	o.tasksMu.Lock()
	o.tasks[taskID] = cancel
	o.tasksMu.Unlock()

	go o.runTaskLoop(taskCtx, taskID, req.Goal, maxSteps)

	return taskID, nil
}

func (o *Orchestrator) CancelTask(taskID string) error {
	o.tasksMu.Lock()
	cancel, exists := o.tasks[taskID]
	delete(o.tasks, taskID)
	o.tasksMu.Unlock()

	if !exists {
		return fmt.Errorf("task '%s' is not actively running", taskID)
	}

	cancel()
	_ = o.db.UpdateTaskStatus(taskID, "CANCELLED", "User cancelled task", 0)
	o.emitEvent(protocol.TaskEvent{
		Type:    protocol.EventTaskFailed,
		TaskID:  taskID,
		Message: "Task was cancelled by user.",
		Success: false,
	})
	return nil
}

func (o *Orchestrator) runTaskLoop(ctx context.Context, taskID, goal string, maxSteps int) {
	start := time.Now()
	_ = o.db.UpdateTaskStatus(taskID, "RUNNING", "", 0)

	o.emitEvent(protocol.TaskEvent{
		Type:    protocol.EventTaskStarted,
		TaskID:  taskID,
		Message: fmt.Sprintf("Started autonomous task execution for goal: '%s'", goal),
		Success: true,
	})

	var history []protocol.TaskEvent

	for step := 1; step <= maxSteps; step++ {
		select {
		case <-ctx.Done():
			_ = o.db.UpdateTaskStatus(taskID, "CANCELLED", "Context cancelled or timed out", step)
			return
		default:
		}

		// 1. Model predicts action
		action, err := o.provider.PredictAction(ctx, goal, history)
		if err != nil {
			log.Printf("[Task %s] Provider prediction error: %v", taskID, err)
			_ = o.db.UpdateTaskStatus(taskID, "FAILED", err.Error(), step)
			o.emitEvent(protocol.TaskEvent{
				Type:    protocol.EventTaskFailed,
				TaskID:  taskID,
				Step:    step,
				Message: fmt.Sprintf("Model error: %v", err),
				Success: false,
			})
			return
		}

		// Check terminal conditions FIRST (do not emit step_started for completed tasks)
		if action.Action == protocol.ActionTaskComplete || action.Action == protocol.ActionSpeak {
			elapsed := time.Since(start).Seconds()
			_ = o.db.UpdateTaskStatus(taskID, "COMPLETED", "", step)
			msg := action.Speech
			if msg == "" {
				msg = action.Thought
			}
			o.emitEvent(protocol.TaskEvent{
				Type:           protocol.EventTaskCompleted,
				TaskID:         taskID,
				Step:           step,
				Action:         action,
				Speech:         action.Speech,
				Thought:        action.Thought,
				Message:        msg,
				Success:        true,
				ElapsedSeconds: elapsed,
			})
			return
		}

		if action.Action == protocol.ActionFail {
			_ = o.db.UpdateTaskStatus(taskID, "FAILED", action.Thought, step)
			o.emitEvent(protocol.TaskEvent{
				Type:    protocol.EventTaskFailed,
				TaskID:  taskID,
				Step:    step,
				Action:  action,
				Message: action.Thought,
				Success: false,
			})
			return
		}

		o.emitEvent(protocol.TaskEvent{
			Type:    protocol.EventStepStarted,
			TaskID:  taskID,
			Step:    step,
			Action:  action,
			Speech:  action.Speech,
			Thought: action.Thought,
			Success: true,
		})

		// 2. Safety Sentinel Policy Check
		check := o.policy.CheckAction(action)
		if !check.Allowed {
			errMsg := fmt.Sprintf("Action blocked by policy: %s", check.Reason)
			_ = o.db.UpdateTaskStatus(taskID, "FAILED", errMsg, step)
			o.emitEvent(protocol.TaskEvent{
				Type:    protocol.EventTaskFailed,
				TaskID:  taskID,
				Step:    step,
				Action:  action,
				Message: errMsg,
				Success: false,
			})
			return
		}

		// 3. Consequential Approval Requirement
		if check.NeedsApproval {
			o.emitEvent(protocol.TaskEvent{
				Type:    protocol.EventApprovalRequested,
				TaskID:  taskID,
				Step:    step,
				Action:  action,
				Thought: fmt.Sprintf("Action requires human confirmation: %s", check.Reason),
				Success: true,
			})

			approved, err := o.approvals.RequestApproval(ctx, taskID, action.Command, check.Reason, check.Severity)
			if err != nil || !approved {
				msg := "Action denied by user or timed out."
				_ = o.db.UpdateTaskStatus(taskID, "CANCELLED", msg, step)
				o.emitEvent(protocol.TaskEvent{
					Type:    protocol.EventTaskFailed,
					TaskID:  taskID,
					Step:    step,
					Message: msg,
					Success: false,
				})
				return
			}
		}

		// 4. Execution across macOS or Virtual Computer
		var rawResult string
		var execErr error
		if isComputerAction(action.Action) {
			rawResult, execErr = o.executeComputerAction(ctx, action)
		} else {
			rawResult, execErr = o.controller.Execute(ctx, action)
		}
		success := execErr == nil

		// 5. Evidence Verification
		verification := o.verifier.VerifyStep(ctx, action, rawResult)
		resultMsg := rawResult
		if !verification.Achieved {
			success = false
			resultMsg = fmt.Sprintf("Verification failed: %s", verification.Description)
		} else if verification.Evidence != "" {
			resultMsg = fmt.Sprintf("%s (Evidence: %s)", rawResult, verification.Evidence)
		}

		// Persist Step
		_ = o.db.RecordStep(taskID, step, string(action.Action), action.Command, action.Thought, resultMsg, success)

		stepEvent := protocol.TaskEvent{
			Type:    protocol.EventStepCompleted,
			TaskID:  taskID,
			Step:    step,
			Action:  action,
			Message: resultMsg,
			Speech:  action.Speech,
			Thought: action.Thought,
			Success: success,
		}
		o.emitEvent(stepEvent)
		history = append(history, stepEvent)
	}

	// Step limit reached
	_ = o.db.UpdateTaskStatus(taskID, "FAILED", "Max step limit exceeded", maxSteps)
	o.emitEvent(protocol.TaskEvent{
		Type:    protocol.EventTaskFailed,
		TaskID:  taskID,
		Message: fmt.Sprintf("Task exceeded maximum step limit of %d steps.", maxSteps),
		Success: false,
	})
}

func isComputerAction(act protocol.ActionType) bool {
	switch act {
	case protocol.ActionComputerStart,
		protocol.ActionComputerStop,
		protocol.ActionComputerStatus,
		protocol.ActionComputerExecute,
		protocol.ActionComputerReadFile,
		protocol.ActionComputerWriteFile,
		protocol.ActionComputerListFiles,
		protocol.ActionComputerOpenApp,
		protocol.ActionComputerScreenshot,
		protocol.ActionComputerInspectDesktop,
		protocol.ActionComputerClick,
		protocol.ActionComputerType,
		protocol.ActionComputerScroll,
		protocol.ActionComputerGetMetrics:
		return true
	default:
		return false
	}
}

func (o *Orchestrator) executeComputerAction(ctx context.Context, action *protocol.AgentAction) (string, error) {
	if o.vmManager == nil {
		return "", fmt.Errorf("virtual computer manager is not initialized")
	}

	cid := action.ComputerID
	if cid == "" {
		cid = "ubuntu-primary"
	}

	switch action.Action {
	case protocol.ActionComputerStart:
		c, err := o.vmManager.StartComputer(ctx, cid)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Virtual computer '%s' started. Status: %s, PID: %d, VNC: :0 (Port %d)", c.Name, c.Status, c.PID, c.VNCPort), nil

	case protocol.ActionComputerStop:
		c, err := o.vmManager.StopComputer(ctx, cid)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Virtual computer '%s' safely stopped. Persistent storage preserved.", c.Name), nil

	case protocol.ActionComputerStatus:
		status, err := o.vmManager.GetComputerStatus(ctx, cid)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Computer '%s' Status: %s, VNC Ready: %v, Agent Connected: %v", status.Computer.Name, status.Computer.Status, status.VNCReady, status.AgentConnected), nil

	case protocol.ActionComputerExecute:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected for computer '%s'", cid)
		}
		res, err := client.ExecCommand(ctx, action.Command, 30, action.Path)
		if err != nil {
			return "", err
		}
		output := res.Stdout
		if res.Stderr != "" {
			output += "\nStderr: " + res.Stderr
		}
		return fmt.Sprintf("Exit Code: %d\nOutput: %s", res.ExitCode, output), nil

	case protocol.ActionComputerReadFile:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		content, err := client.ReadFile(ctx, action.Path, "utf-8")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("File content of '%s':\n%s", action.Path, content), nil

	case protocol.ActionComputerWriteFile:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		err := client.WriteFile(ctx, action.Path, action.Content, "utf-8", 0644)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("File successfully written to '%s'", action.Path), nil

	case protocol.ActionComputerListFiles:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		files, err := client.ListFiles(ctx, action.Path)
		if err != nil {
			return "", err
		}
		var list []string
		for _, f := range files {
			typeStr := "file"
			if f.IsDir {
				typeStr = "dir"
			}
			list = append(list, fmt.Sprintf("%s (%s, %d bytes)", f.Name, typeStr, f.Size))
		}
		return fmt.Sprintf("Files in '%s':\n%s", action.Path, fmt.Sprint(list)), nil

	case protocol.ActionComputerOpenApp:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		err := client.LaunchApp(ctx, action.AppName, nil)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Application '%s' launched inside virtual computer.", action.AppName), nil

	case protocol.ActionComputerScreenshot:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		shot, err := client.CaptureScreenshot(ctx)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Screenshot captured (%dx%d %s)", shot.Width, shot.Height, shot.Format), nil

	case protocol.ActionComputerInspectDesktop:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		apps, _ := client.ListApps(ctx)
		return fmt.Sprintf("Desktop inspected. Active apps discovered: %d", len(apps)), nil

	case protocol.ActionComputerClick:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		x, y := 640, 400
		if action.X != nil {
			x = *action.X
		}
		if action.Y != nil {
			y = *action.Y
		}
		err := client.SendInput(ctx, "click", x, y, "", "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Clicked virtual computer at (%d, %d)", x, y), nil

	case protocol.ActionComputerType:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		err := client.SendInput(ctx, "type", 0, 0, action.Text, "")
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Typed text '%s' into virtual computer", action.Text), nil

	case protocol.ActionComputerScroll:
		client := o.vmManager.GetClient(cid)
		if client == nil {
			return "", fmt.Errorf("guest agent client not connected")
		}
		err := client.SendInput(ctx, "scroll", 0, 0, "", action.ScrollDirection)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Scrolled %s in virtual computer", action.ScrollDirection), nil

	case protocol.ActionComputerGetMetrics:
		metrics, err := o.vmManager.GetComputerMetrics(ctx, cid)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("CPU: %.1f%%, Memory: %.1fMB / %.1fMB, Disk: %.1fMB / %.1fMB",
			metrics.CPUPercent, metrics.MemoryUsedMB, metrics.MemoryTotalMB, metrics.DiskUsedMB, metrics.DiskTotalMB), nil

	default:
		return "", fmt.Errorf("unknown computer action: %s", action.Action)
	}
}

func (o *Orchestrator) AnalyzeScreen(ctx context.Context, imgB64, activeApp, winTitle, ocrText, prompt string) (string, error) {
	if metaProv, ok := o.provider.(*providers.MetaMuseProvider); ok {
		return metaProv.AnalyzeScreenVision(ctx, imgB64, activeApp, winTitle, ocrText, prompt)
	}
	return o.controller.InspectScreen(ctx)
}
