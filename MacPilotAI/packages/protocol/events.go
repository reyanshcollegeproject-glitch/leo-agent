package protocol

import "time"

type ActionType string

const (
	ActionClick        ActionType = "click"
	ActionDoubleClick  ActionType = "double_click"
	ActionRightClick   ActionType = "right_click"
	ActionTypeText     ActionType = "type_text"
	ActionPressHotkey  ActionType = "press_hotkey"
	ActionLaunchApp    ActionType = "launch_app"
	ActionRunCommand   ActionType = "run_command"
	ActionWait         ActionType = "wait"
	ActionTaskComplete ActionType = "task_complete"
	ActionFail         ActionType = "fail"
)

type AgentAction struct {
	Action       ActionType `json:"action"`
	Thought      string     `json:"thought,omitempty"`
	X            *int       `json:"x,omitempty"`
	Y            *int       `json:"y,omitempty"`
	ElementID    *int       `json:"element_id,omitempty"`
	Text         string     `json:"text,omitempty"`
	Hotkey       string     `json:"hotkey,omitempty"`
	AppName      string     `json:"app_name,omitempty"`
	Command      string     `json:"command,omitempty"`
	Seconds      float64    `json:"seconds,omitempty"`
}

type TaskRequest struct {
	ID        string `json:"id"`
	Goal      string `json:"goal"`
	Model     string `json:"model,omitempty"`
	MaxSteps  int    `json:"max_steps,omitempty"`
	StreamWS  bool   `json:"stream_ws,omitempty"`
}

type EventType string

const (
	EventTaskStarted       EventType = "task_started"
	EventStepStarted       EventType = "step_started"
	EventStepCompleted     EventType = "step_completed"
	EventApprovalRequested EventType = "approval_requested"
	EventApprovalResolved  EventType = "approval_resolved"
	EventTaskCompleted     EventType = "task_completed"
	EventTaskFailed        EventType = "task_failed"
)

type TaskEvent struct {
	Type           EventType    `json:"type"`
	TaskID         string       `json:"task_id"`
	Step           int          `json:"step,omitempty"`
	Action         *AgentAction `json:"action,omitempty"`
	Message        string       `json:"message,omitempty"`
	Thought        string       `json:"thought,omitempty"`
	Success        bool         `json:"success"`
	ElapsedSeconds float64      `json:"elapsed_seconds,omitempty"`
	Timestamp      time.Time    `json:"timestamp"`
}

type ApprovalRequest struct {
	ID        string    `json:"id"`
	TaskID    string    `json:"task_id"`
	Command   string    `json:"command"`
	Reason    string    `json:"reason"`
	Severity  string    `json:"severity"` // "high", "critical", "medium"
	CreatedAt time.Time `json:"created_at"`
}

type ApprovalResponse struct {
	ID       string `json:"id"`
	Approved bool   `json:"approved"`
	Feedback string `json:"feedback,omitempty"`
}

