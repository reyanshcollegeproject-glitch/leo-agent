package protocol

import "time"

type ActionType string

const (
	ActionClick        ActionType = "click"
	ActionDoubleClick  ActionType = "double_click"
	ActionTripleClick  ActionType = "triple_click"
	ActionRightClick   ActionType = "right_click"
	ActionMiddleClick  ActionType = "middle_click"
	ActionMouseMove    ActionType = "mouse_move"
	ActionMouseDrag    ActionType = "mouse_drag"
	ActionScroll       ActionType = "scroll"
	ActionTypeText     ActionType = "type_text"
	ActionPressHotkey  ActionType = "press_hotkey"
	ActionKey          ActionType = "key"
	ActionLaunchApp      ActionType = "launch_app"
	ActionRunCommand     ActionType = "run_command"
	ActionWebSearch      ActionType = "web_search"
	ActionBrowseURL      ActionType = "browse_url"
	ActionFindFiles      ActionType = "find_files"
	ActionSystemStatus   ActionType = "system_status"
	ActionTakeScreenshot ActionType = "take_screenshot"
	ActionInspectScreen  ActionType = "inspect_screen"
	ActionSpeak          ActionType = "speak"
	ActionWait           ActionType = "wait"
	ActionTaskComplete   ActionType = "task_complete"
	ActionFail           ActionType = "fail"

	// Dedicated Virtual Computer Actions
	ActionComputerStart          ActionType = "computer_start"
	ActionComputerStop           ActionType = "computer_stop"
	ActionComputerStatus         ActionType = "computer_status"
	ActionComputerExecute        ActionType = "computer_execute_command"
	ActionComputerReadFile       ActionType = "computer_read_file"
	ActionComputerWriteFile      ActionType = "computer_write_file"
	ActionComputerListFiles      ActionType = "computer_list_files"
	ActionComputerOpenApp        ActionType = "computer_open_application"
	ActionComputerScreenshot     ActionType = "computer_capture_screenshot"
	ActionComputerInspectDesktop ActionType = "computer_inspect_desktop"
	ActionComputerClick          ActionType = "computer_click_element"
	ActionComputerType           ActionType = "computer_type_text"
	ActionComputerScroll         ActionType = "computer_scroll"
	ActionComputerGetMetrics     ActionType = "computer_get_metrics"
)

type AgentAction struct {
	Action          ActionType `json:"action"`
	Speech          string     `json:"speech,omitempty"`
	Thought         string     `json:"thought,omitempty"`
	Query           string     `json:"query,omitempty"`
	URL             string     `json:"url,omitempty"`
	Path            string     `json:"path,omitempty"`
	X               *int       `json:"x,omitempty"`
	Y               *int       `json:"y,omitempty"`
	StartX          *int       `json:"start_x,omitempty"`
	StartY          *int       `json:"start_y,omitempty"`
	ScrollDirection string     `json:"scroll_direction,omitempty"`
	ScrollAmount    int        `json:"scroll_amount,omitempty"`
	ElementID       *int       `json:"element_id,omitempty"`
	Text            string     `json:"text,omitempty"`
	Hotkey          string     `json:"hotkey,omitempty"`
	Key             string     `json:"key,omitempty"`
	AppName         string     `json:"app_name,omitempty"`
	Command         string     `json:"command,omitempty"`
	Content         string     `json:"content,omitempty"`
	ComputerID      string     `json:"computer_id,omitempty"`
	Seconds         float64    `json:"seconds,omitempty"`
	Mode            string     `json:"mode,omitempty"`
}

type TaskRequest struct {
	ID        string `json:"id"`
	Goal      string `json:"goal"`
	Model     string `json:"model,omitempty"`
	Mode      string `json:"mode,omitempty"`
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
	Speech         string       `json:"speech,omitempty"`
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
