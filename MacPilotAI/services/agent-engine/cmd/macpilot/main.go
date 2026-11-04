package main

import (
	"flag"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/macpilot/agent-engine/internal/approvals"
	"github.com/macpilot/agent-engine/internal/gateway"
	"github.com/macpilot/agent-engine/internal/orchestrator"
	"github.com/macpilot/agent-engine/internal/persistence"
	"github.com/macpilot/agent-engine/internal/providers"
	"github.com/macpilot/agent-engine/internal/vm"
)

func loadEnvFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		return
	}
	lines := strings.Split(string(data), "\n")
	for _, raw := range lines {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) == 2 {
			k := strings.TrimSpace(parts[0])
			v := strings.Trim(strings.TrimSpace(parts[1]), `"'`)
			if os.Getenv(k) == "" {
				os.Setenv(k, v)
			}
		}
	}
}

func main() {
	log.Println("STEP 1: Main started")
	port := flag.Int("port", 8089, "Port to listen on for Gateway HTTP/WebSocket")
	defaultDB := filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "MacPilot", "macpilot.db")
	dbPath := flag.String("db", defaultDB, "SQLite database file path")
	controlFlag := flag.String("control-model", "", "Meta AI Control model name")
	talkFlag := flag.String("talk-model", "", "Meta AI Talk model name")
	flag.Parse()

	log.Println("STEP 2: Loading env files")
	loadEnvFile(".env")
	loadEnvFile("../.env")
	loadEnvFile("../../.env")
	loadEnvFile("/Users/reyansh/Desktop/contribute/auto agent/.env")
	execPath, _ := os.Executable()
	if execPath != "" {
		bundleEnv := filepath.Join(filepath.Dir(execPath), "..", ".env")
		loadEnvFile(bundleEnv)
	}
	appSupportEnv := filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "MacPilot", ".env")
	loadEnvFile(appSupportEnv)

	log.Println("STEP 3: Checking DB directory")
	if dir := filepath.Dir(*dbPath); dir != "." && dir != "" {
		_ = os.MkdirAll(dir, 0755)
	}

	log.Println("STEP 4: Initializing SQLite DB at", *dbPath)
	db, err := persistence.NewDB(*dbPath)
	if err != nil {
		log.Fatalf("Database initialization failed: %v", err)
	}
	defer db.Close()

	log.Println("STEP 5: Initializing Approval Manager")
	apprManager := approvals.NewManager(db)

	log.Println("STEP 6: Initializing Model Provider (Dual Meta AI Models: Control & Talk)")
	apiKey := os.Getenv("MUSE_API_KEY")
	endpoint := os.Getenv("MUSE_ENDPOINT")
	if endpoint == "" {
		endpoint = "https://api.meta.ai/v1/chat/completions"
	}
	controlModel := os.Getenv("MUSE_CONTROL_MODEL")
	if *controlFlag != "" {
		controlModel = *controlFlag
	}
	if controlModel == "" {
		controlModel = "muse-wonder-1.0"
	}
	talkModel := os.Getenv("MUSE_TALK_MODEL")
	if *talkFlag != "" {
		talkModel = *talkFlag
	}
	if talkModel == "" {
		talkModel = "muse-spark-1.3"
	}
	log.Printf("🤖 Dual Meta AI Models initialized: Control=%s | Talk=%s (Endpoint: %s)", controlModel, talkModel, endpoint)
	provider := providers.NewMetaMuseProvider(apiKey, controlModel, talkModel, endpoint)

	log.Println("STEP 7: Initializing Virtual Computer Manager")
	vmBaseDir := filepath.Join(os.Getenv("HOME"), "Library", "Application Support", "MacPilot", "vm")
	vmManager := vm.NewComputerManager(db, vmBaseDir)

	log.Println("STEP 8: Initializing Core Orchestrator")
	orch := orchestrator.NewOrchestrator(db, provider, apprManager, vmManager)

	log.Println("STEP 9: Initializing Gateway Server")
	server := gateway.NewServer(orch, apprManager, vmManager, *port)

	stopChan := make(chan os.Signal, 1)
	signal.Notify(stopChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-stopChan
		log.Println("\n🛑 Gracefully shutting down MacPilotAI Agent Engine...")
		_ = db.Close()
		os.Exit(0)
	}()

	log.Printf("STEP 9: Starting Gateway Server on port %d...", *port)
	if err := server.Start(); err != nil {
		log.Fatalf("Server stopped: %v", err)
	}
}

