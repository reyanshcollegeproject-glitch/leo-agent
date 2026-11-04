package vm

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
	"github.com/macpilot/agent-engine/internal/persistence"
)

type ComputerManager struct {
	db          *persistence.DB
	baseDir     string
	provisioner *Provisioner
	computers   map[string]*Computer
	processes   map[string]*exec.Cmd
	clients     map[string]*GuestClient
	mu          sync.RWMutex
}

func NewComputerManager(db *persistence.DB, baseDir string) *ComputerManager {
	if baseDir == "" {
		baseDir = filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "MacPilot", "vm")
	}
	_ = os.MkdirAll(baseDir, 0755)

	cacheDir := filepath.Join(baseDir, "cache")
	provisioner := NewProvisioner(cacheDir)

	mgr := &ComputerManager{
		db:          db,
		baseDir:     baseDir,
		provisioner: provisioner,
		computers:   make(map[string]*Computer),
		processes:   make(map[string]*exec.Cmd),
		clients:     make(map[string]*GuestClient),
	}

	// Load existing persistent computer if any
	mgr.loadPersistentComputer()
	return mgr
}

func (m *ComputerManager) loadPersistentComputer() {
	m.mu.Lock()
	defer m.mu.Unlock()

	defaultID := "ubuntu-primary"
	diskPath := filepath.Join(m.baseDir, defaultID, "disk.qcow2")
	isoPath := filepath.Join(m.baseDir, defaultID, "cidata.iso")

	if _, err := os.Stat(diskPath); err == nil {
		token := "ef104d1b75f98750575756874331d6df"
		if isoBytes, err := os.ReadFile(isoPath); err == nil {
			str := string(isoBytes)
			if idx := strings.Index(str, "--token "); idx != -1 {
				sub := str[idx+len("--token "):]
				f := strings.Fields(sub)
				if len(f) > 0 {
					token = f[0]
				}
			}
		}

		c := &Computer{
			ID:             defaultID,
			Name:           "Ubuntu 24.04 ARM64",
			OS:             "Ubuntu 24.04 LTS (ARM64)",
			Status:         StatusStopped,
			CPUCores:       4,
			MemoryMB:       4096,
			DiskSizeGB:     20,
			DiskPath:       diskPath,
			SeedISOPath:    isoPath,
			GuestAgentPort: 8090,
			VNCPort:        5900,
			SSHPort:        2222,
			Token:          token,
			ControlMode:    "ai",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		m.computers[defaultID] = c
		m.clients[defaultID] = NewGuestClient(c.GuestAgentPort, c.Token)
		log.Printf("🖥️ Loaded persistent computer profile: %s (%s)", c.Name, c.ID)
	}
}

func (m *ComputerManager) GetPrimaryComputer() *Computer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, c := range m.computers {
		return c
	}
	return nil
}

func (m *ComputerManager) GetClient(id string) *GuestClient {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if id == "" {
		for _, c := range m.clients {
			return c
		}
	}
	return m.clients[id]
}

func (m *ComputerManager) CreateComputer(ctx context.Context, req CreateComputerRequest) (*Computer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	id := "ubuntu-primary"
	if req.Name == "" {
		req.Name = "Ubuntu 24.04 ARM64"
	}
	if req.CPUCores <= 0 {
		req.CPUCores = 4
	}
	if req.MemoryMB <= 0 {
		req.MemoryMB = 4096
	}
	if req.DiskSizeGB <= 0 {
		req.DiskSizeGB = 20
	}

	vmDir := filepath.Join(m.baseDir, id)
	_ = os.MkdirAll(vmDir, 0755)

	diskPath := filepath.Join(vmDir, "disk.qcow2")
	isoPath := filepath.Join(vmDir, "cidata.iso")

	// 1. Ensure Base Ubuntu Minimal ARM64 Image exists
	baseImg, err := m.provisioner.EnsureBaseImage(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to ensure base cloud image: %w", err)
	}

	// 2. Create Persistent Sparse Overlay Disk
	if err := m.provisioner.CreateOverlayDisk(baseImg, diskPath, req.DiskSizeGB); err != nil {
		return nil, fmt.Errorf("failed to create overlay disk: %w", err)
	}

	// 3. Generate Secret Token
	tokenBytes := make([]byte, 16)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)

	// 4. Generate Cloud-Init Seed ISO
	if err := m.provisioner.GenerateCloudInitISO(isoPath, id, "macpilot-ubuntu", "macpilot", "macpilot", token); err != nil {
		return nil, fmt.Errorf("failed to generate cloud-init ISO: %w", err)
	}

	c := &Computer{
		ID:             id,
		Name:           req.Name,
		OS:             "Ubuntu 24.04 LTS (ARM64)",
		Status:         StatusStopped,
		CPUCores:       req.CPUCores,
		MemoryMB:       req.MemoryMB,
		DiskSizeGB:     req.DiskSizeGB,
		DiskPath:       diskPath,
		SeedISOPath:    isoPath,
		GuestAgentPort: 8090,
		VNCPort:        5900,
		SSHPort:        2222,
		Token:          token,
		ControlMode:    "ai",
		CreatedAt:      time.Now().UTC(),
		UpdatedAt:      time.Now().UTC(),
	}

	m.computers[id] = c
	m.clients[id] = NewGuestClient(c.GuestAgentPort, c.Token)
	log.Printf("🎉 Computer '%s' (%s) successfully provisioned!", c.Name, c.ID)
	return c, nil
}

func (m *ComputerManager) StartComputer(ctx context.Context, id string) (*Computer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		id = "ubuntu-primary"
	}

	c, ok := m.computers[id]
	if !ok {
		// Auto-provision if primary doesn't exist yet
		m.mu.Unlock()
		newC, err := m.CreateComputer(ctx, CreateComputerRequest{})
		m.mu.Lock()
		if err != nil {
			return nil, err
		}
		c = newC
	}

	// Check if process already running
	if cmd, exists := m.processes[id]; exists && cmd.Process != nil {
		if err := cmd.Process.Signal(syscall.Signal(0)); err == nil {
			c.Status = StatusRunning
			return c, nil
		}
	}

	c.Status = StatusStarting
	c.UpdatedAt = time.Now().UTC()

	// Verify firmware exists
	if _, err := os.Stat(UEFIFirmwarePath); err != nil {
		return nil, fmt.Errorf("QEMU ARM64 UEFI firmware not found at %s: %w", UEFIFirmwarePath, err)
	}

	args := []string{
		"-M", "virt,highmem=on",
		"-accel", "hvf",
		"-cpu", "host",
		"-smp", fmt.Sprintf("%d", c.CPUCores),
		"-m", fmt.Sprintf("%d", c.MemoryMB),
		"-bios", UEFIFirmwarePath,
		"-drive", fmt.Sprintf("file=%s,if=virtio,format=qcow2", c.DiskPath),
		"-device", "virtio-gpu-pci",
		"-device", "virtio-tablet-pci",
		"-device", "virtio-keyboard-pci",
		"-vnc", "127.0.0.1:0", // Port 5900
		"-netdev", fmt.Sprintf("user,id=net0,hostfwd=tcp:127.0.0.1:%d-:22,hostfwd=tcp:127.0.0.1:%d-:8090", c.SSHPort, c.GuestAgentPort),
		"-device", "virtio-net-pci,netdev=net0",
		"-display", "none",
	}

	// Attach seed ISO if present
	if _, err := os.Stat(c.SeedISOPath); err == nil {
		args = append(args, "-drive", fmt.Sprintf("file=%s,if=virtio,format=raw", c.SeedISOPath))
	}

	log.Printf("🚀 Starting QEMU Virtual Machine: %s %v", QEMUBinaryPath, args)
	cmd := exec.Command(QEMUBinaryPath, args...)

	logFile := filepath.Join(m.baseDir, id, "qemu.log")
	if out, err := os.OpenFile(logFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644); err == nil {
		cmd.Stdout = out
		cmd.Stderr = out
	}

	if err := cmd.Start(); err != nil {
		c.Status = StatusError
		return nil, fmt.Errorf("failed to start QEMU: %w", err)
	}

	c.PID = cmd.Process.Pid
	c.Status = StatusRunning
	m.processes[id] = cmd

	log.Printf("⚡ QEMU VM started with PID: %d (VNC: :0 on port %d, Guest Agent: %d)", c.PID, c.VNCPort, c.GuestAgentPort)
	return c, nil
}

func (m *ComputerManager) StopComputer(ctx context.Context, id string) (*Computer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		id = "ubuntu-primary"
	}

	c, ok := m.computers[id]
	if !ok {
		return nil, fmt.Errorf("computer '%s' not found", id)
	}

	c.Status = StatusStopping
	cmd, exists := m.processes[id]
	if exists && cmd.Process != nil {
		// Attempt graceful ACPI shutdown via guest agent
		client := m.clients[id]
		if client != nil {
			_, _ = client.ExecCommand(ctx, "shutdown -h now", 5, "/")
		}

		// Wait briefly or terminate QEMU process
		done := make(chan error, 1)
		go func() { done <- cmd.Wait() }()

		select {
		case <-done:
			log.Printf("🛑 QEMU VM stopped gracefully.")
		case <-time.After(3 * time.Second):
			_ = cmd.Process.Signal(syscall.SIGTERM)
			select {
			case <-done:
			case <-time.After(2 * time.Second):
				_ = cmd.Process.Kill()
			}
		}
		delete(m.processes, id)
	}

	c.Status = StatusStopped
	c.PID = 0
	c.UpdatedAt = time.Now().UTC()
	log.Printf("🛑 Computer '%s' stopped. Persistent disk preserved.", id)
	return c, nil
}

func (m *ComputerManager) RestartComputer(ctx context.Context, id string) (*Computer, error) {
	if _, err := m.StopComputer(ctx, id); err != nil {
		log.Printf("Warning during stop on restart: %v", err)
	}
	time.Sleep(1 * time.Second)
	return m.StartComputer(ctx, id)
}

func (m *ComputerManager) DeleteComputer(ctx context.Context, id string, confirm bool) error {
	if !confirm {
		return fmt.Errorf("deletion rejected: explicit confirmation (confirm=true) is required to delete virtual computer and its data")
	}

	_, _ = m.StopComputer(ctx, id)

	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		id = "ubuntu-primary"
	}

	vmDir := filepath.Join(m.baseDir, id)
	_ = os.RemoveAll(vmDir)
	delete(m.computers, id)
	delete(m.clients, id)
	delete(m.processes, id)

	log.Printf("🗑️ Virtual computer '%s' and persistent disk deleted.", id)
	return nil
}

func (m *ComputerManager) GetComputerStatus(ctx context.Context, id string) (*ComputerStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if id == "" {
		id = "ubuntu-primary"
	}

	c, ok := m.computers[id]
	if !ok {
		diskPath := filepath.Join(m.baseDir, id, "disk.qcow2")
		isoPath := filepath.Join(m.baseDir, id, "cidata.iso")
		c = &Computer{
			ID:             id,
			Name:           "Ubuntu 24.04 ARM64",
			OS:             "Ubuntu 24.04 LTS (ARM64)",
			Status:         StatusStopped,
			CPUCores:       4,
			MemoryMB:       4096,
			DiskSizeGB:     20,
			DiskPath:       diskPath,
			SeedISOPath:    isoPath,
			GuestAgentPort: 8090,
			VNCPort:        5900,
			SSHPort:        2222,
			Token:          "ef104d1b75f98750575756874331d6df",
			ControlMode:    "ai",
			CreatedAt:      time.Now().UTC(),
			UpdatedAt:      time.Now().UTC(),
		}
		m.computers[id] = c
		m.clients[id] = NewGuestClient(c.GuestAgentPort, c.Token)
	}

	status := &ComputerStatus{
		Computer: *c,
	}

	// Check VNC Port accessibility
	vncConn, err := net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", c.VNCPort), 200*time.Millisecond)
	if err == nil {
		status.VNCReady = true
		_ = vncConn.Close()
	}

	// Check Guest Agent connection
	client := m.clients[id]
	if client != nil {
		healthCtx, cancel := context.WithTimeout(ctx, 300*time.Millisecond)
		defer cancel()
		alive, _ := client.Health(healthCtx)
		status.AgentConnected = alive
	}

	return status, nil
}

func (m *ComputerManager) GetComputerMetrics(ctx context.Context, id string) (*ComputerMetrics, error) {
	client := m.GetClient(id)
	if client == nil {
		return nil, fmt.Errorf("guest client not available for computer '%s'", id)
	}
	return client.GetMetrics(ctx)
}

func (m *ComputerManager) CreateSnapshot(ctx context.Context, id, name, description string) (*Snapshot, error) {
	m.mu.RLock()
	c, ok := m.computers[id]
	m.mu.RUnlock()
	if !ok {
		return nil, fmt.Errorf("computer '%s' not found", id)
	}

	if name == "" {
		name = fmt.Sprintf("snap-%d", time.Now().Unix())
	}

	log.Printf("📸 Creating QEMU disk snapshot '%s' for computer '%s'...", name, id)
	cmd := exec.Command(QEMUImgPath, "snapshot", "-c", name, c.DiskPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("qemu-img snapshot failed: %s (%w)", string(out), err)
	}

	snap := &Snapshot{
		ID:          uuid.New().String(),
		ComputerID:  id,
		Name:        name,
		Description: description,
		CreatedAt:   time.Now().UTC(),
	}
	log.Printf("✅ Snapshot '%s' created successfully.", name)
	return snap, nil
}

func (m *ComputerManager) RestoreSnapshot(ctx context.Context, id, name string) error {
	m.mu.RLock()
	c, ok := m.computers[id]
	m.mu.RUnlock()
	if !ok {
		return fmt.Errorf("computer '%s' not found", id)
	}

	// Must stop VM before snapshot restoration
	_, _ = m.StopComputer(ctx, id)

	log.Printf("⏪ Restoring QEMU disk snapshot '%s' for computer '%s'...", name, id)
	cmd := exec.Command(QEMUImgPath, "snapshot", "-a", name, c.DiskPath)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("qemu-img restore failed: %s (%w)", string(out), err)
	}

	log.Printf("✅ Snapshot '%s' restored successfully.", name)
	return nil
}

func (m *ComputerManager) SetControlMode(id, mode string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, ok := m.computers[id]
	if !ok {
		return fmt.Errorf("computer '%s' not found", id)
	}
	c.ControlMode = mode
	return nil
}

