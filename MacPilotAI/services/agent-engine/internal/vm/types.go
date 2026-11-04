package vm

import "time"

type ComputerStatusType string

const (
	StatusStopped  ComputerStatusType = "STOPPED"
	StatusStarting ComputerStatusType = "STARTING"
	StatusRunning  ComputerStatusType = "RUNNING"
	StatusStopping ComputerStatusType = "STOPPING"
	StatusError    ComputerStatusType = "ERROR"
)

type Computer struct {
	ID             string             `json:"id"`
	Name           string             `json:"name"`
	OS             string             `json:"os"`
	Status         ComputerStatusType `json:"status"`
	CPUCores       int                `json:"cpu_cores"`
	MemoryMB       int                `json:"memory_mb"`
	DiskSizeGB     int                `json:"disk_size_gb"`
	DiskPath       string             `json:"disk_path"`
	SeedISOPath    string             `json:"seed_iso_path"`
	GuestAgentPort int                `json:"guest_agent_port"`
	VNCPort        int                `json:"vnc_port"`
	SSHPort        int                `json:"ssh_port"`
	Token          string             `json:"token"`
	PID            int                `json:"pid"`
	ControlMode    string             `json:"control_mode"` // "ai" or "manual"
	CreatedAt      time.Time          `json:"created_at"`
	UpdatedAt      time.Time          `json:"updated_at"`
}

type ComputerStatus struct {
	Computer       Computer `json:"computer"`
	UptimeSeconds  float64  `json:"uptime_seconds"`
	AgentConnected bool     `json:"agent_connected"`
	VNCReady       bool     `json:"vnc_ready"`
	ActiveProcesses int     `json:"active_processes"`
}

type ComputerMetrics struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryUsedMB  float64   `json:"memory_used_mb"`
	MemoryTotalMB float64   `json:"memory_total_mb"`
	DiskUsedMB    float64   `json:"disk_used_mb"`
	DiskTotalMB   float64   `json:"disk_total_mb"`
	Timestamp     time.Time `json:"timestamp"`
}

type Snapshot struct {
	ID          string    `json:"id"`
	ComputerID  string    `json:"computer_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
}

type CreateComputerRequest struct {
	Name       string `json:"name"`
	CPUCores   int    `json:"cpu_cores,omitempty"`
	MemoryMB   int    `json:"memory_mb,omitempty"`
	DiskSizeGB int    `json:"disk_size_gb,omitempty"`
}

