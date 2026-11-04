package main

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/google/uuid"
)

var (
	authToken string
	runningCmds   = make(map[string]context.CancelFunc)
	runningCmdsMu sync.Mutex
)

type ExecRequest struct {
	Command        string `json:"command"`
	TimeoutSeconds int    `json:"timeout_seconds"`
	Cwd            string `json:"cwd"`
	Background     bool   `json:"background"`
}

type ExecResponse struct {
	ID         string `json:"id"`
	Stdout     string `json:"stdout"`
	Stderr     string `json:"stderr"`
	ExitCode   int    `json:"exit_code"`
	DurationMs int64  `json:"duration_ms"`
	Success    bool   `json:"success"`
}

type WriteFileRequest struct {
	Path    string `json:"path"`
	Content string `json:"content"`
	Append  bool   `json:"append"`
	Mode    uint32 `json:"mode"`
}

type DesktopInputRequest struct {
	Action    string `json:"action"` // click, double_click, right_click, type, key, scroll
	X         int    `json:"x"`
	Y         int    `json:"y"`
	Text      string `json:"text"`
	Key       string `json:"key"`
	Direction string `json:"direction"`
	Amount    int    `json:"amount"`
}

type MetricsResponse struct {
	CPUPercent float64 `json:"cpu_percent"`
	RAMUsedMB  uint64  `json:"ram_used_mb"`
	RAMTotalMB uint64  `json:"ram_total_mb"`
	DiskUsedGB float64 `json:"disk_used_gb"`
	DiskTotalGB float64 `json:"disk_total_gb"`
	UptimeSec  int64   `json:"uptime_sec"`
}

func authMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if authToken != "" {
			authHeader := r.Header.Get("Authorization")
			expected := "Bearer " + authToken
			if authHeader != expected {
				http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
				return
			}
		}
		w.Header().Set("Content-Type", "application/json")
		next(w, r)
	}
}

func main() {
	port := flag.Int("port", 8090, "Port for Guest Agent server")
	tokenFlag := flag.String("token", "", "Shared secret token for authentication")
	flag.Parse()

	if *tokenFlag != "" {
		authToken = *tokenFlag
	} else if envTok := os.Getenv("MACPILOT_GUEST_TOKEN"); envTok != "" {
		authToken = envTok
	}

	mux := http.NewServeMux()

	// 1. Health & Status
	mux.HandleFunc("/api/v1/health", handleHealth)

	// 2. Command Execution & Cancellation
	mux.HandleFunc("/api/v1/commands/exec", authMiddleware(handleExec))
	mux.HandleFunc("/api/v1/commands/cancel", authMiddleware(handleCancel))

	// 3. Filesystem Operations
	mux.HandleFunc("/api/v1/fs/read", authMiddleware(handleFSRead))
	mux.HandleFunc("/api/v1/fs/write", authMiddleware(handleFSWrite))
	mux.HandleFunc("/api/v1/fs/list", authMiddleware(handleFSList))

	// 4. Applications & Processes
	mux.HandleFunc("/api/v1/apps", authMiddleware(handleApps))
	mux.HandleFunc("/api/v1/apps/launch", authMiddleware(handleAppLaunch))
	mux.HandleFunc("/api/v1/processes", authMiddleware(handleProcesses))

	// 5. Desktop Interaction & Screenshots
	mux.HandleFunc("/api/v1/desktop/screenshot", authMiddleware(handleScreenshot))
	mux.HandleFunc("/api/v1/desktop/input", authMiddleware(handleDesktopInput))

	// 6. Resource Telemetry
	mux.HandleFunc("/api/v1/metrics", authMiddleware(handleMetrics))

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	log.Printf("🤖 MacPilot Ubuntu Guest Agent listening on %s (Auth enabled: %v)", addr, authToken != "")
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("Guest Agent error: %v", err)
	}
}

func handleHealth(w http.ResponseWriter, r *http.Request) {
	hostname, _ := os.Hostname()
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ready",
		"service":  "MacPilot Ubuntu Guest Agent",
		"version":  "1.0.0",
		"hostname": hostname,
		"time":     time.Now().UTC(),
	})
}

func handleExec(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST required"}`, http.StatusMethodNotAllowed)
		return
	}

	var req ExecRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"invalid JSON: %s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	cmdStr := strings.TrimSpace(req.Command)
	if cmdStr == "" {
		http.Error(w, `{"error":"command cannot be empty"}`, http.StatusBadRequest)
		return
	}

	// Least-privilege and safety checks
	lower := strings.ToLower(cmdStr)
	if strings.Contains(lower, "mkfs") || strings.Contains(lower, "dd if=/dev/zero") || strings.Contains(lower, ":(){ :|:& };:") {
		http.Error(w, `{"error":"destructive command blocked by policy"}`, http.StatusForbidden)
		return
	}

	timeout := 60 * time.Second
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	cmdID := uuid.New().String()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)

	runningCmdsMu.Lock()
	runningCmds[cmdID] = cancel
	runningCmdsMu.Unlock()

	defer func() {
		runningCmdsMu.Lock()
		delete(runningCmds, cmdID)
		runningCmdsMu.Unlock()
		cancel()
	}()

	cmd := exec.CommandContext(ctx, "bash", "-c", cmdStr)
	if req.Cwd != "" {
		cmd.Dir = req.Cwd
	}

	cmd.Env = append(os.Environ(), "DISPLAY=:0", "TERM=xterm-256color")

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	start := time.Now()
	err := cmd.Run()
	duration := time.Since(start).Milliseconds()

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			exitCode = 1
			stderr.WriteString(fmt.Sprintf("\nExecution error: %v", err))
		}
	}

	resp := ExecResponse{
		ID:         cmdID,
		Stdout:     stdout.String(),
		Stderr:     stderr.String(),
		ExitCode:   exitCode,
		DurationMs: duration,
		Success:    exitCode == 0,
	}

	_ = json.NewEncoder(w).Encode(resp)
}

func handleCancel(w http.ResponseWriter, r *http.Request) {
	cmdID := r.URL.Query().Get("id")
	if cmdID == "" {
		http.Error(w, `{"error":"id parameter required"}`, http.StatusBadRequest)
		return
	}

	runningCmdsMu.Lock()
	cancel, exists := runningCmds[cmdID]
	runningCmdsMu.Unlock()

	if !exists {
		http.Error(w, `{"error":"command not found or already finished"}`, http.StatusNotFound)
		return
	}

	cancel()
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "cancelled", "id": cmdID})
}

func handleFSRead(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" && r.Body != nil {
		var req struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		path = req.Path
	}
	if path == "" {
		http.Error(w, `{"error":"path required"}`, http.StatusBadRequest)
		return
	}

	clean := filepath.Clean(path)
	data, err := os.ReadFile(clean)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    clean,
		"content": string(data),
		"size":    len(data),
	})
}

func handleFSWrite(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST required"}`, http.StatusMethodNotAllowed)
		return
	}

	var req WriteFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	clean := filepath.Clean(req.Path)
	if err := os.MkdirAll(filepath.Dir(clean), 0755); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	mode := os.FileMode(0644)
	if req.Mode > 0 {
		mode = os.FileMode(req.Mode)
	}

	flag := os.O_CREATE | os.O_WRONLY
	if req.Append {
		flag |= os.O_APPEND
	} else {
		flag |= os.O_TRUNC
	}

	f, err := os.OpenFile(clean, flag, mode)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}
	defer f.Close()

	n, err := f.WriteString(req.Content)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success":       true,
		"path":          clean,
		"bytes_written": n,
	})
}

func handleFSList(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" && r.Body != nil {
		var req struct {
			Path string `json:"path"`
		}
		_ = json.NewDecoder(r.Body).Decode(&req)
		if req.Path != "" {
			path = req.Path
		}
	}
	if path == "" {
		path = "."
	}

	clean := filepath.Clean(path)
	entries, err := os.ReadDir(clean)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	type Entry struct {
		Name    string `json:"name"`
		IsDir   bool   `json:"is_dir"`
		Size    int64  `json:"size"`
		ModTime string `json:"mod_time"`
	}

	var results []Entry
	for _, e := range entries {
		info, err := e.Info()
		var size int64
		var modTime string
		if err == nil {
			size = info.Size()
			modTime = info.ModTime().Format(time.RFC3339)
		}
		results = append(results, Entry{
			Name:    e.Name(),
			IsDir:   e.IsDir(),
			Size:    size,
			ModTime: modTime,
		})
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"path":    clean,
		"entries": results,
		"count":   len(results),
	})
}

func handleApps(w http.ResponseWriter, r *http.Request) {
	dirs := []string{"/usr/share/applications", "/usr/local/share/applications"}
	if home := os.Getenv("HOME"); home != "" {
		dirs = append(dirs, filepath.Join(home, ".local", "share", "applications"))
	}

	type AppInfo struct {
		Name    string `json:"name"`
		Exec    string `json:"exec"`
		Comment string `json:"comment"`
		Icon    string `json:"icon"`
	}

	var apps []AppInfo
	seen := make(map[string]bool)

	for _, d := range dirs {
		files, err := os.ReadDir(d)
		if err != nil {
			continue
		}
		for _, f := range files {
			if strings.HasSuffix(f.Name(), ".desktop") {
				content, err := os.ReadFile(filepath.Join(d, f.Name()))
				if err != nil {
					continue
				}
				name, execCmd, comment, icon := parseDesktopFile(string(content))
				if name != "" && !seen[name] {
					seen[name] = true
					apps = append(apps, AppInfo{
						Name:    name,
						Exec:    execCmd,
						Comment: comment,
						Icon:    icon,
					})
				}
			}
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"apps":  apps,
		"count": len(apps),
	})
}

func parseDesktopFile(content string) (name, execCmd, comment, icon string) {
	lines := strings.Split(content, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Name=") && name == "" {
			name = strings.TrimPrefix(l, "Name=")
		} else if strings.HasPrefix(l, "Exec=") && execCmd == "" {
			execCmd = strings.TrimPrefix(l, "Exec=")
		} else if strings.HasPrefix(l, "Comment=") && comment == "" {
			comment = strings.TrimPrefix(l, "Comment=")
		} else if strings.HasPrefix(l, "Icon=") && icon == "" {
			icon = strings.TrimPrefix(l, "Icon=")
		}
	}
	return
}

func handleAppLaunch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST required"}`, http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		App  string   `json:"app"`
		Args []string `json:"args"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	cmdName := req.App
	switch strings.ToLower(req.App) {
	case "chromium", "browser", "chrome":
		cmdName = "chromium-browser"
	case "terminal":
		cmdName = "xfce4-terminal"
	}

	cmd := exec.Command(cmdName, req.Args...)
	cmd.Env = append(os.Environ(), "DISPLAY=:0")
	if err := cmd.Start(); err != nil {
		// Fallback to nohup or xdg-open
		cmdFallback := exec.Command("sh", "-c", fmt.Sprintf("nohup %s >/dev/null 2>&1 &", cmdName))
		cmdFallback.Env = append(os.Environ(), "DISPLAY=:0")
		if err2 := cmdFallback.Run(); err2 != nil {
			http.Error(w, fmt.Sprintf(`{"error":"failed to launch app: %v"}`, err2), http.StatusInternalServerError)
			return
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"app":     req.App,
		"status":  "launched",
	})
}

func handleProcesses(w http.ResponseWriter, r *http.Request) {
	cmd := exec.Command("ps", "-eo", "pid,user,%cpu,%mem,comm", "--sort=-%cpu")
	out, err := cmd.Output()
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	lines := strings.Split(strings.TrimSpace(string(out)), "\n")
	type Proc struct {
		PID     string `json:"pid"`
		User    string `json:"user"`
		CPU     string `json:"cpu"`
		Mem     string `json:"mem"`
		Command string `json:"command"`
	}

	var procs []Proc
	for i, l := range lines {
		if i == 0 {
			continue // skip header
		}
		fields := strings.Fields(l)
		if len(fields) >= 5 {
			procs = append(procs, Proc{
				PID:     fields[0],
				User:    fields[1],
				CPU:     fields[2],
				Mem:     fields[3],
				Command: fields[4],
			})
		}
		if len(procs) >= 30 {
			break
		}
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"processes": procs,
		"count":     len(procs),
	})
}

func handleScreenshot(w http.ResponseWriter, r *http.Request) {
	tmpPath := fmt.Sprintf("/tmp/screenshot_%d.png", time.Now().UnixNano())
	defer os.Remove(tmpPath)

	// Try scrot, import, or xwd
	cmd := exec.Command("sh", "-c", fmt.Sprintf("scrot %s 2>/dev/null || import -window root %s 2>/dev/null", tmpPath, tmpPath))
	cmd.Env = append(os.Environ(), "DISPLAY=:0")
	if err := cmd.Run(); err != nil || !fileExists(tmpPath) {
		// Fallback dummy or headless check
		http.Error(w, `{"error":"failed to capture X11 screenshot (display :0 unavailable or tools missing)"}`, http.StatusInternalServerError)
		return
	}

	data, err := os.ReadFile(tmpPath)
	if err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusInternalServerError)
		return
	}

	b64 := base64.StdEncoding.EncodeToString(data)
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"format":  "png",
		"base64":  b64,
		"size":    len(data),
	})
}

func handleDesktopInput(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, `{"error":"POST required"}`, http.StatusMethodNotAllowed)
		return
	}

	var req DesktopInputRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"%s"}`, err.Error()), http.StatusBadRequest)
		return
	}

	var xdotoolArgs []string
	switch req.Action {
	case "click":
		xdotoolArgs = []string{"mousemove", fmt.Sprintf("%d", req.X), fmt.Sprintf("%d", req.Y), "click", "1"}
	case "double_click":
		xdotoolArgs = []string{"mousemove", fmt.Sprintf("%d", req.X), fmt.Sprintf("%d", req.Y), "click", "--repeat", "2", "1"}
	case "right_click":
		xdotoolArgs = []string{"mousemove", fmt.Sprintf("%d", req.X), fmt.Sprintf("%d", req.Y), "click", "3"}
	case "type":
		xdotoolArgs = []string{"type", "--delay", "12", req.Text}
	case "key":
		xdotoolArgs = []string{"key", req.Key}
	case "scroll":
		btn := "5" // scroll down
		if req.Direction == "up" {
			btn = "4"
		}
		amt := req.Amount
		if amt <= 0 {
			amt = 3
		}
		xdotoolArgs = []string{"click", "--repeat", fmt.Sprintf("%d", amt), btn}
	default:
		http.Error(w, `{"error":"unknown desktop action"}`, http.StatusBadRequest)
		return
	}

	cmd := exec.Command("xdotool", xdotoolArgs...)
	cmd.Env = append(os.Environ(), "DISPLAY=:0")
	if err := cmd.Run(); err != nil {
		http.Error(w, fmt.Sprintf(`{"error":"xdotool error: %v"}`, err), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{"success": true, "action": req.Action})
}

func handleMetrics(w http.ResponseWriter, r *http.Request) {
	var metrics MetricsResponse

	// 1. RAM info from /proc/meminfo
	if memData, err := os.ReadFile("/proc/meminfo"); err == nil {
		lines := strings.Split(string(memData), "\n")
		var totalKB, availKB uint64
		for _, l := range lines {
			if strings.HasPrefix(l, "MemTotal:") {
				fmt.Sscanf(l, "MemTotal: %d kB", &totalKB)
			} else if strings.HasPrefix(l, "MemAvailable:") {
				fmt.Sscanf(l, "MemAvailable: %d kB", &availKB)
			}
		}
		metrics.RAMTotalMB = totalKB / 1024
		if totalKB > availKB {
			metrics.RAMUsedMB = (totalKB - availKB) / 1024
		}
	}

	// 2. Disk info via Statfs
	var stat syscall.Statfs_t
	if err := syscall.Statfs("/", &stat); err == nil {
		totalBytes := stat.Blocks * uint64(stat.Bsize)
		freeBytes := stat.Bfree * uint64(stat.Bsize)
		metrics.DiskTotalGB = float64(totalBytes) / (1024 * 1024 * 1024)
		metrics.DiskUsedGB = float64(totalBytes-freeBytes) / (1024 * 1024 * 1024)
	}

	// 3. CPU percent
	metrics.CPUPercent = getCPUPercent()

	// 4. Uptime from /proc/uptime
	if upData, err := os.ReadFile("/proc/uptime"); err == nil {
		var sec float64
		fmt.Sscanf(string(upData), "%f", &sec)
		metrics.UptimeSec = int64(sec)
	}

	_ = json.NewEncoder(w).Encode(metrics)
}

func getCPUPercent() float64 {
	readStat := func() (idle, total uint64) {
		data, err := os.ReadFile("/proc/stat")
		if err != nil {
			return 0, 0
		}
		lines := strings.Split(string(data), "\n")
		if len(lines) == 0 {
			return 0, 0
		}
		fields := strings.Fields(lines[0])
		if len(fields) < 5 {
			return 0, 0
		}
		var user, nice, sys, id, iowait, irq, softirq uint64
		fmt.Sscanf(fields[1], "%d", &user)
		fmt.Sscanf(fields[2], "%d", &nice)
		fmt.Sscanf(fields[3], "%d", &sys)
		fmt.Sscanf(fields[4], "%d", &id)
		if len(fields) > 5 {
			fmt.Sscanf(fields[5], "%d", &iowait)
		}
		if len(fields) > 6 {
			fmt.Sscanf(fields[6], "%d", &irq)
		}
		if len(fields) > 7 {
			fmt.Sscanf(fields[7], "%d", &softirq)
		}
		idle = id + iowait
		total = user + nice + sys + idle + irq + softirq
		return
	}

	idle1, total1 := readStat()
	time.Sleep(100 * time.Millisecond)
	idle2, total2 := readStat()

	diffIdle := float64(idle2 - idle1)
	diffTotal := float64(total2 - total1)
	if diffTotal <= 0 {
		return 0.0
	}
	cpuUsage := (1.0 - (diffIdle / diffTotal)) * 100.0
	if cpuUsage < 0 {
		return 0.0
	}
	if cpuUsage > 100.0 {
		return 100.0
	}
	return cpuUsage
}

func fileExists(p string) bool {
	_, err := os.Stat(p)
	return err == nil
}
