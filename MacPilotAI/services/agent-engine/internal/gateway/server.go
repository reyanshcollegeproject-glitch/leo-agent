package gateway

import (
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
	"github.com/macpilot/agent-engine/internal/approvals"
	"github.com/macpilot/agent-engine/internal/orchestrator"
	"github.com/macpilot/agent-engine/internal/vm"
	"github.com/macpilot/agent-engine/pkg/protocol"
)

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool { return true }, // Local loopback IPC
}

type Server struct {
	orchestrator *orchestrator.Orchestrator
	approvals    *approvals.Manager
	vmManager    *vm.ComputerManager
	wsClients    map[*websocket.Conn]bool
	wsMu         sync.Mutex
	port         int
}

func NewServer(
	orch *orchestrator.Orchestrator,
	appr *approvals.Manager,
	vmManager *vm.ComputerManager,
	port int,
) *Server {
	s := &Server{
		orchestrator: orch,
		approvals:    appr,
		vmManager:    vmManager,
		wsClients:    make(map[*websocket.Conn]bool),
		port:         port,
	}

	// Subscribe to orchestrator event bus and broadcast via WebSocket
	orch.Subscribe(func(event protocol.TaskEvent) {
		s.broadcastEvent(event)
	})

	return s
}

func (s *Server) broadcastEvent(event protocol.TaskEvent) {
	bytes, err := json.Marshal(event)
	if err != nil {
		return
	}

	s.wsMu.Lock()
	defer s.wsMu.Unlock()

	for conn := range s.wsClients {
		if err := conn.WriteMessage(websocket.TextMessage, bytes); err != nil {
			conn.Close()
			delete(s.wsClients, conn)
		}
	}
}

func (s *Server) Start() error {
	mux := http.NewServeMux()

	// Core API
	mux.HandleFunc("/api/v1/health", s.handleHealth)
	mux.HandleFunc("/api/v1/tasks", s.handleTasks)
	mux.HandleFunc("/api/v1/tasks/cancel", s.handleCancelTask)
	mux.HandleFunc("/api/v1/approvals", s.handleApprovals)
	mux.HandleFunc("/api/v1/vision/analyze", s.handleVisionAnalyze)
	mux.HandleFunc("/ws/events", s.handleWebSocket)

	// Virtual Computer API
	mux.HandleFunc("/api/v1/computers", s.handleComputers)
	mux.HandleFunc("/api/v1/computers/start", s.handleComputerStart)
	mux.HandleFunc("/api/v1/computers/stop", s.handleComputerStop)
	mux.HandleFunc("/api/v1/computers/restart", s.handleComputerRestart)
	mux.HandleFunc("/api/v1/computers/delete", s.handleComputerDelete)
	mux.HandleFunc("/api/v1/computers/status", s.handleComputerStatus)
	mux.HandleFunc("/api/v1/computers/metrics", s.handleComputerMetrics)
	mux.HandleFunc("/api/v1/computers/mode", s.handleComputerMode)
	mux.HandleFunc("/api/v1/computers/snapshots", s.handleComputerSnapshots)
	mux.HandleFunc("/api/v1/computers/snapshots/restore", s.handleComputerRestoreSnapshot)
	mux.HandleFunc("/api/v1/guest-agent/binary", s.handleGuestAgentBinary)

	// Live Desktop WebSocket Proxy
	mux.HandleFunc("/ws/vnc", s.handleVNCWebSocket)

	// Serve noVNC static files
	novncDir := s.resolveNoVNCDir()
	if novncDir != "" {
		fileServer := http.FileServer(http.Dir(novncDir))
		mux.Handle("/desktop/", http.StripPrefix("/desktop/", fileServer))
		log.Printf("🖥️ Serving noVNC Desktop Client from %s at /desktop/", novncDir)
	}

	addr := fmt.Sprintf("127.0.0.1:%d", s.port)
	log.Printf("🚀 MacPilotAI Agent Gateway listening on http://%s", addr)
	return http.ListenAndServe(addr, mux)
}

func (s *Server) resolveNoVNCDir() string {
	candidates := []string{
		"web/novnc",
		"services/agent-engine/web/novnc",
		"/Users/reyansh/Desktop/contribute/auto agent/MacPilotAI/services/agent-engine/web/novnc",
	}
	execPath, err := os.Executable()
	if err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), "..", "web", "novnc"))
		candidates = append(candidates, filepath.Join(filepath.Dir(execPath), "web", "novnc"))
	}
	for _, dir := range candidates {
		if fi, err := os.Stat(dir); err == nil && fi.IsDir() {
			return dir
		}
	}
	return ""
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "healthy",
		"service": "MacPilotAI Agent Gateway",
		"version": "1.0.0",
	})
}

type VisionAnalysisRequest struct {
	Image       string `json:"image"`
	ActiveApp   string `json:"active_app"`
	WindowTitle string `json:"window_title"`
	OCRText     string `json:"ocr_text"`
	Prompt      string `json:"prompt"`
}

func (s *Server) handleVisionAnalyze(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req VisionAnalysisRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	analysis, err := s.orchestrator.AnalyzeScreen(r.Context(), req.Image, req.ActiveApp, req.WindowTitle, req.OCRText, req.Prompt)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"status":   "ok",
		"analysis": analysis,
	})
}

func (s *Server) handleTasks(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodPost {
		var req protocol.TaskRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		taskID, err := s.orchestrator.SubmitTask(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"task_id": taskID,
			"status":  "submitted",
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleCancelTask(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "missing 'id' query parameter", http.StatusBadRequest)
		return
	}

	if err := s.orchestrator.CancelTask(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	_ = json.NewEncoder(w).Encode(map[string]interface{}{
		"id":     id,
		"status": "cancelled",
	})
}

func (s *Server) handleApprovals(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if r.Method == http.MethodGet {
		pending := s.approvals.ListPending()
		_ = json.NewEncoder(w).Encode(pending)
		return
	}

	if r.Method == http.MethodPost {
		id := r.URL.Query().Get("id")
		approved := strings.ToLower(r.URL.Query().Get("approved")) == "true"

		if id == "" {
			http.Error(w, "missing 'id' query parameter", http.StatusBadRequest)
			return
		}

		if err := s.approvals.Resolve(id, approved); err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]interface{}{
			"id":       id,
			"approved": approved,
			"status":   "resolved",
		})
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("WebSocket upgrade failed: %v", err)
		return
	}

	s.wsMu.Lock()
	s.wsClients[conn] = true
	s.wsMu.Unlock()

	log.Printf("🔌 Client connected to MacPilotAI event stream.")

	for {
		if _, _, err := conn.ReadMessage(); err != nil {
			s.wsMu.Lock()
			delete(s.wsClients, conn)
			s.wsMu.Unlock()
			conn.Close()
			break
		}
	}
}

// MARK: - Virtual Computer Handlers

func (s *Server) handleComputers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	if s.vmManager == nil {
		http.Error(w, "Computer manager not initialized", http.StatusServiceUnavailable)
		return
	}

	if r.Method == http.MethodGet {
		comp := s.vmManager.GetPrimaryComputer()
		if comp == nil {
			_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "NONE"})
			return
		}
		_ = json.NewEncoder(w).Encode(comp)
		return
	}

	if r.Method == http.MethodPost {
		var req vm.CreateComputerRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		comp, err := s.vmManager.CreateComputer(r.Context(), req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(comp)
		return
	}

	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleComputerStart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	comp, err := s.vmManager.StartComputer(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(comp)
}

func (s *Server) handleComputerStop(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	comp, err := s.vmManager.StopComputer(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(comp)
}

func (s *Server) handleComputerRestart(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	comp, err := s.vmManager.RestartComputer(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(comp)
}

func (s *Server) handleComputerDelete(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	confirm := strings.ToLower(r.URL.Query().Get("confirm")) == "true"
	if !confirm {
		http.Error(w, "explicit confirm=true is required to delete virtual computer", http.StatusBadRequest)
		return
	}

	if err := s.vmManager.DeleteComputer(r.Context(), id, true); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "deleted", "id": id})
}

func (s *Server) handleComputerStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	status, err := s.vmManager.GetComputerStatus(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(status)
}

func (s *Server) handleComputerMetrics(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	metrics, err := s.vmManager.GetComputerMetrics(r.Context(), id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusServiceUnavailable)
		return
	}
	_ = json.NewEncoder(w).Encode(metrics)
}

func (s *Server) handleComputerMode(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	mode := r.URL.Query().Get("mode") // "ai" or "manual"
	if mode == "" {
		http.Error(w, "mode parameter is required", http.StatusBadRequest)
		return
	}
	if err := s.vmManager.SetControlMode(id, mode); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"mode": mode})
}

func (s *Server) handleComputerSnapshots(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	if r.Method == http.MethodPost {
		name := r.URL.Query().Get("name")
		desc := r.URL.Query().Get("description")
		snap, err := s.vmManager.CreateSnapshot(r.Context(), id, name, desc)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		_ = json.NewEncoder(w).Encode(snap)
		return
	}
	http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
}

func (s *Server) handleComputerRestoreSnapshot(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	id := r.URL.Query().Get("id")
	name := r.URL.Query().Get("name")
	if name == "" {
		http.Error(w, "snapshot name is required", http.StatusBadRequest)
		return
	}
	if err := s.vmManager.RestoreSnapshot(r.Context(), id, name); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	_ = json.NewEncoder(w).Encode(map[string]interface{}{"status": "restored", "name": name})
}

func (s *Server) handleGuestAgentBinary(w http.ResponseWriter, r *http.Request) {
	candidates := []string{
		"services/guest-agent/bin/macpilot-guest-agent",
		"../guest-agent/bin/macpilot-guest-agent",
		"/Users/reyansh/Desktop/contribute/auto agent/MacPilotAI/services/guest-agent/bin/macpilot-guest-agent",
	}
	for _, p := range candidates {
		if fi, err := os.Stat(p); err == nil && !fi.IsDir() {
			w.Header().Set("Content-Type", "application/octet-stream")
			http.ServeFile(w, r, p)
			return
		}
	}
	http.Error(w, "macpilot-guest-agent binary not found", http.StatusNotFound)
}

// MARK: - Native Go noVNC WebSocket Proxy

func (s *Server) handleVNCWebSocket(w http.ResponseWriter, r *http.Request) {
	wsConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("VNC WebSocket upgrade failed: %v", err)
		return
	}
	defer wsConn.Close()

	tcpConn, err := net.DialTimeout("tcp", "127.0.0.1:5900", 2*time.Second)
	if err != nil {
		log.Printf("VNC TCP dial to 127.0.0.1:5900 failed: %v", err)
		_ = wsConn.WriteMessage(websocket.TextMessage, []byte("VNC server is offline or unreachable"))
		return
	}
	defer tcpConn.Close()

	done := make(chan struct{}, 2)

	// ws -> tcp
	go func() {
		defer func() { done <- struct{}{} }()
		for {
			msgType, data, err := wsConn.ReadMessage()
			if err != nil {
				return
			}
			if msgType == websocket.BinaryMessage || msgType == websocket.TextMessage {
				if _, err := tcpConn.Write(data); err != nil {
					return
				}
			}
		}
	}()

	// tcp -> ws
	go func() {
		defer func() { done <- struct{}{} }()
		buf := make([]byte, 32768)
		for {
			n, err := tcpConn.Read(buf)
			if err != nil {
				return
			}
			if err := wsConn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				return
			}
		}
	}()

	<-done
}
