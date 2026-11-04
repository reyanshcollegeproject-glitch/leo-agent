package providers

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/macpilot/agent-engine/pkg/protocol"
)

type Provider interface {
	PredictAction(ctx context.Context, goal string, history []protocol.TaskEvent) (*protocol.AgentAction, error)
}

type MetaMuseProvider struct {
	APIKey       string
	ControlModel string // muse-wonder-1.0 (Flagship frontier reasoning for OS & VM computer control)
	TalkModel    string // muse-spark-1.3 (Fast conversational agent for voice & natural dialogue)
	Endpoint     string
	HTTPClient   *http.Client
}

func NewMetaMuseProvider(apiKey, controlModel, talkModel, endpoint string) *MetaMuseProvider {
	if apiKey == "" {
		apiKey = os.Getenv("MUSE_API_KEY")
	}
	if controlModel == "" {
		controlModel = os.Getenv("MUSE_CONTROL_MODEL")
		if controlModel == "" {
			controlModel = os.Getenv("MUSE_MODEL_NAME")
			if controlModel == "" {
				controlModel = "muse-wonder-1.0"
			}
		}
	}
	if talkModel == "" {
		talkModel = os.Getenv("MUSE_TALK_MODEL")
		if talkModel == "" {
			talkModel = "muse-spark-1.3"
		}
	}
	if endpoint == "" {
		endpoint = os.Getenv("MUSE_ENDPOINT")
		if endpoint == "" {
			endpoint = "https://api.meta.ai/v1/chat/completions"
		}
	}
	return &MetaMuseProvider{
		APIKey:       apiKey,
		ControlModel: controlModel,
		TalkModel:    talkModel,
		Endpoint:     endpoint,
		HTTPClient:   &http.Client{Timeout: 35 * time.Second},
	}
}

func (p *MetaMuseProvider) PredictAction(ctx context.Context, goal string, history []protocol.TaskEvent) (*protocol.AgentAction, error) {
	goalLower := strings.ToLower(strings.TrimSpace(goal))

	// Fast Screen Vision Route (Single-pass, zero looping)
	if isVisionQuery(goalLower) {
		if len(history) == 0 {
			return &protocol.AgentAction{
				Action:  protocol.ActionInspectScreen,
				Speech:  "Looking at your screen now.",
				Thought: "Screen inspection requested.",
			}, nil
		} else if len(history) > 0 && history[len(history)-1].Action != nil && history[len(history)-1].Action.Action == protocol.ActionInspectScreen {
			spoken := p.summarizeScreenObservation(history[len(history)-1].Message)
			return &protocol.AgentAction{
				Action:  protocol.ActionTaskComplete,
				Speech:  spoken,
				Thought: "Screen analysis complete.",
			}, nil
		}
	}

	// 1. Primary: Dual Meta AI Models (Control & Talk)
	if p.APIKey != "" {
		// If initial step and query is conversational, route directly to Meta Talk Model (muse-spark-1.3)
		if len(history) == 0 && isConversationalQuery(goal) {
			log.Printf("🗣️ [Meta Multi-Model] Routing conversational goal to Talk Model (%s)...", p.TalkModel)
			if act, err := p.callTalkModel(ctx, goal); err == nil && act != nil {
				return act, nil
			}
		}

		// Otherwise, route to Meta Control Model (muse-wonder-1.0) for deep reasoning & OS/VM computer execution
		log.Printf("🕹️ [Meta Multi-Model] Routing computer task to Control Model (%s)...", p.ControlModel)
		act, err := p.callControlModel(ctx, goal, history)
		if err == nil && act != nil {
			// When Control Model finishes a task that observed tool output, use the Talk Model to polish the vocal response!
			if act.Action == protocol.ActionTaskComplete && len(history) > 0 {
				lastOutput := history[len(history)-1].Message
				if lastOutput != "" {
					log.Printf("🗣️ [Meta Multi-Model] Synthesizing conversational speech via Talk Model (%s)...", p.TalkModel)
					if spokenSummary, err := p.synthesizeSpokenSummary(ctx, goal, lastOutput); err == nil && spokenSummary != "" {
						act.Speech = spokenSummary
					}
				}
			}
			return act, nil
		}
		log.Printf("⚠️ Meta AI models failed or returned nil, using offline handler: %v", err)
	}

	// 2. Secondary: Offline Direct Intents & Fast Fallback Handlers
	return p.predictOfflineAction(goal, history)
}

func isVisionQuery(g string) bool {
	return strings.Contains(g, "look at my screen") || strings.Contains(g, "what is on my screen") ||
		strings.Contains(g, "what's on my screen") || strings.Contains(g, "see my screen") ||
		strings.Contains(g, "what do you see") || strings.Contains(g, "can you see") ||
		strings.Contains(g, "analyze screen") || strings.Contains(g, "see macbook screen") ||
		strings.Contains(g, "describe screen") || strings.Contains(g, "read screen") ||
		strings.Contains(g, "tell me what you see") || strings.Contains(g, "what am i looking at") ||
		strings.Contains(g, "what is on screen")
}

func isConversationalQuery(goal string) bool {
	g := strings.ToLower(strings.TrimSpace(goal))
	if g == "" {
		return true
	}
	// Action keywords that require OS or VM control
	actionKeywords := []string{
		"open", "launch", "run", "search", "battery", "status", "computer", "vm",
		"virtual", "screenshot", "screen", "look", "see", "vision", "volume", "mute", "unmute",
		"click", "double click", "right click", "move", "cursor", "mouse", "drag", "scroll",
		"type", "press", "key", "hotkey", "play", "song",
		"find", "kill", "stop", "restart", "reboot", "exec", "terminal", "safari",
		"chrome", "finder", "notes", "code", "vscode", "curl", "git", "bash", "ls", "cd", "window", "windows",
	}
	for _, kw := range actionKeywords {
		if strings.Contains(g, kw) {
			return false
		}
	}
	return true
}

func (p *MetaMuseProvider) predictOfflineAction(goal string, history []protocol.TaskEvent) (*protocol.AgentAction, error) {
	// If a step has already executed, conclude with the output message (never loop infinitely!)
	if len(history) > 0 {
		lastStep := history[len(history)-1]
		if lastStep.Action != nil {
			act := lastStep.Action.Action
			if act == protocol.ActionLaunchApp {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  fmt.Sprintf("%s is open and ready.", lastStep.Action.AppName),
					Thought: fmt.Sprintf("%s launched successfully.", lastStep.Action.AppName),
				}, nil
			}
			if act == protocol.ActionRunCommand {
				msg := lastStep.Message
				if msg == "" {
					msg = "Action completed."
				}
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  fmt.Sprintf("Done: %s", msg),
					Thought: "Run command completed successfully.",
				}, nil
			}
			if act == protocol.ActionMouseMove {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Mouse cursor moved.",
					Thought: "Mouse movement completed.",
				}, nil
			}
			if act == protocol.ActionClick {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Clicked.",
					Thought: "Mouse click completed.",
				}, nil
			}
			if act == protocol.ActionDoubleClick {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Double clicked.",
					Thought: "Mouse double click completed.",
				}, nil
			}
			if act == protocol.ActionRightClick {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Right clicked.",
					Thought: "Mouse right click completed.",
				}, nil
			}
			if act == protocol.ActionMouseDrag {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Dragged across screen.",
					Thought: "Mouse drag completed.",
				}, nil
			}
			if act == protocol.ActionScroll {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Scrolled page.",
					Thought: "Mouse scroll completed.",
				}, nil
			}
			if act == protocol.ActionTypeText {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Text typed successfully.",
					Thought: "Text entry completed.",
				}, nil
			}
			if act == protocol.ActionKey || act == protocol.ActionPressHotkey {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Key shortcut pressed.",
					Thought: "Keyboard shortcut executed.",
				}, nil
			}
			if act == protocol.ActionSystemStatus {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  lastStep.Message,
					Thought: "System status check completed.",
				}, nil
			}
			if act == protocol.ActionTakeScreenshot {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Screen captured successfully.",
					Thought: "Screenshot completed.",
				}, nil
			}
			if act == protocol.ActionInspectScreen {
				spoken := p.summarizeScreenObservation(lastStep.Message)
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  spoken,
					Thought: fmt.Sprintf("Computer vision analysis complete: %s", spoken),
				}, nil
			}
			if act == protocol.ActionWebSearch {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  fmt.Sprintf("Web search completed for %s.", lastStep.Action.Query),
					Thought: "Web search completed.",
				}, nil
			}
			if act == protocol.ActionComputerStart {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Your virtual computer is fully booted and ready.",
					Thought: "Computer started.",
				}, nil
			}
			if act == protocol.ActionComputerStop {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Virtual computer safely powered down. Persistent storage preserved.",
					Thought: "Computer stopped.",
				}, nil
			}
			if act == protocol.ActionComputerStatus || act == protocol.ActionComputerGetMetrics {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Virtual computer telemetry retrieved.",
					Thought: "Computer telemetry retrieved.",
				}, nil
			}
			if act == protocol.ActionComputerExecute {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  fmt.Sprintf("Command result: %s", lastStep.Message),
					Thought: "Virtual command executed.",
				}, nil
			}
			if act == protocol.ActionComputerReadFile || act == protocol.ActionComputerWriteFile {
				return &protocol.AgentAction{
					Action:  protocol.ActionTaskComplete,
					Speech:  "Filesystem operation inside the virtual computer complete.",
					Thought: "Virtual filesystem operation complete.",
				}, nil
			}
		}
		msg := lastStep.Message
		if msg == "" {
			msg = "Task finished."
		}
		return &protocol.AgentAction{
			Action:  protocol.ActionTaskComplete,
			Speech:  msg,
			Thought: "Completed task execution.",
		}, nil
	}

	goalLower := strings.ToLower(strings.TrimSpace(goal))

	// Hardware & Virtual Computer Direct Intents (Only run on step 0!)
	isComputerTarget := strings.Contains(goalLower, "computer") || strings.Contains(goalLower, "vm") || strings.Contains(goalLower, "virtual machine")

	if isComputerTarget && (strings.Contains(goalLower, "start") || strings.Contains(goalLower, "boot") || strings.Contains(goalLower, "turn on") || strings.Contains(goalLower, "power on") || strings.Contains(goalLower, "launch")) {
		return &protocol.AgentAction{
			Action:  protocol.ActionComputerStart,
			Speech:  "Starting the virtual computer.",
			Thought: "Starting virtual computer via QEMU.",
		}, nil
	}

	if isComputerTarget && (strings.Contains(goalLower, "stop") || strings.Contains(goalLower, "shutdown") || strings.Contains(goalLower, "turn off") || strings.Contains(goalLower, "power off") || strings.Contains(goalLower, "halt")) {
		return &protocol.AgentAction{
			Action:  protocol.ActionComputerStop,
			Speech:  "Stopping the virtual computer safely.",
			Thought: "Stopping virtual computer.",
		}, nil
	}

	if isComputerTarget && (strings.Contains(goalLower, "status") || strings.Contains(goalLower, "check") || strings.Contains(goalLower, "running") || strings.Contains(goalLower, "health")) {
		return &protocol.AgentAction{
			Action:  protocol.ActionComputerStatus,
			Speech:  "Checking virtual computer status.",
			Thought: "Querying virtual computer status.",
		}, nil
	}

	if isComputerTarget && (strings.Contains(goalLower, "metric") || strings.Contains(goalLower, "telemetry") || strings.Contains(goalLower, "resource") || strings.Contains(goalLower, "cpu") || strings.Contains(goalLower, "ram") || strings.Contains(goalLower, "memory")) {
		return &protocol.AgentAction{
			Action:  protocol.ActionComputerGetMetrics,
			Speech:  "Checking virtual computer metrics.",
			Thought: "Querying virtual computer metrics.",
		}, nil
	}

	if isComputerTarget && (strings.Contains(goalLower, "screenshot") || strings.Contains(goalLower, "capture") || strings.Contains(goalLower, "screen")) {
		return &protocol.AgentAction{
			Action:  protocol.ActionComputerScreenshot,
			Speech:  "Capturing virtual desktop screenshot.",
			Thought: "Capturing virtual computer screenshot.",
		}, nil
	}

	if strings.HasPrefix(goalLower, "run in computer ") || strings.HasPrefix(goalLower, "execute in computer ") {
		cmd := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(goalLower, "run in computer "), "execute in computer "))
		return &protocol.AgentAction{
			Action:  protocol.ActionComputerExecute,
			Command: cmd,
			Speech:  fmt.Sprintf("Running '%s' in Ubuntu.", cmd),
			Thought: fmt.Sprintf("Running command '%s' inside guest agent.", cmd),
		}, nil
	}

	// Web Search heuristic
	if strings.HasPrefix(goalLower, "search for") || strings.HasPrefix(goalLower, "google ") || strings.HasPrefix(goalLower, "search the web for") {
		query := goalLower
		for _, prefix := range []string{"search the web for", "search for", "google "} {
			if strings.HasPrefix(query, prefix) {
				query = strings.TrimSpace(strings.TrimPrefix(query, prefix))
				break
			}
		}
		if query != "" {
			return &protocol.AgentAction{
				Action:  protocol.ActionWebSearch,
				Query:   query,
				Speech:  fmt.Sprintf("Searching for %s.", query),
				Thought: fmt.Sprintf("Web search requested for '%s'.", query),
			}, nil
		}
	}

	// Mouse & Cursor Direct Control Intents
	if strings.Contains(goalLower, "move mouse") || strings.Contains(goalLower, "move cursor") ||
		strings.Contains(goalLower, "cursor to") || strings.Contains(goalLower, "mouse to") {
		x, y := 640, 400
		var speech string

		coordRegex := regexp.MustCompile(`(?i)(?:move\s+(?:mouse|cursor)\s+to\s+)?(\d{1,4})[,\s]+(\d{1,4})`)
		if matches := coordRegex.FindStringSubmatch(goalLower); len(matches) > 2 {
			var parsedX, parsedY int
			fmt.Sscanf(matches[1], "%d", &parsedX)
			fmt.Sscanf(matches[2], "%d", &parsedY)
			x, y = parsedX, parsedY
			speech = fmt.Sprintf("Moving cursor to coordinate %d, %d.", x, y)
		} else if strings.Contains(goalLower, "top left") || strings.Contains(goalLower, "top-left") {
			x, y = 100, 100
			speech = "Moving cursor to top left."
		} else if strings.Contains(goalLower, "top right") || strings.Contains(goalLower, "top-right") {
			x, y = 1180, 100
			speech = "Moving cursor to top right."
		} else if strings.Contains(goalLower, "bottom left") || strings.Contains(goalLower, "bottom-left") {
			x, y = 100, 700
			speech = "Moving cursor to bottom left."
		} else if strings.Contains(goalLower, "bottom right") || strings.Contains(goalLower, "bottom-right") {
			x, y = 1180, 700
			speech = "Moving cursor to bottom right."
		} else if strings.Contains(goalLower, "center") || strings.Contains(goalLower, "middle") {
			x, y = 640, 400
			speech = "Moving cursor to center of screen."
		} else {
			x, y = 640, 400
			speech = "Moving cursor to center."
		}

		return &protocol.AgentAction{
			Action:  protocol.ActionMouseMove,
			X:       &x,
			Y:       &y,
			Speech:  speech,
			Thought: fmt.Sprintf("Direct mouse movement to %d, %d.", x, y),
		}, nil
	}

	// Mouse Clicks
	if strings.Contains(goalLower, "triple click") {
		x, y := 640, 400
		return &protocol.AgentAction{
			Action:  protocol.ActionTripleClick,
			X:       &x,
			Y:       &y,
			Speech:  "Triple clicking mouse.",
			Thought: "Physical triple click requested.",
		}, nil
	}
	if strings.Contains(goalLower, "double click") {
		x, y := 640, 400
		return &protocol.AgentAction{
			Action:  protocol.ActionDoubleClick,
			X:       &x,
			Y:       &y,
			Speech:  "Double clicking mouse.",
			Thought: "Physical double click requested.",
		}, nil
	}
	if strings.Contains(goalLower, "right click") {
		x, y := 640, 400
		return &protocol.AgentAction{
			Action:  protocol.ActionRightClick,
			X:       &x,
			Y:       &y,
			Speech:  "Right clicking mouse.",
			Thought: "Physical right click requested.",
		}, nil
	}
	if goalLower == "click" || goalLower == "left click" || strings.HasPrefix(goalLower, "click ") ||
		strings.Contains(goalLower, "click here") || strings.Contains(goalLower, "click mouse") {
		x, y := 640, 400
		if strings.Contains(goalLower, "center") {
			x, y = 640, 400
		} else if strings.Contains(goalLower, "top left") {
			x, y = 100, 100
		}
		return &protocol.AgentAction{
			Action:  protocol.ActionClick,
			X:       &x,
			Y:       &y,
			Speech:  "Clicking mouse.",
			Thought: "Physical left click requested.",
		}, nil
	}

	// Mouse Scroll
	if strings.Contains(goalLower, "scroll down") {
		return &protocol.AgentAction{
			Action:          protocol.ActionScroll,
			ScrollDirection: "down",
			ScrollAmount:    5,
			Speech:          "Scrolling down.",
			Thought:         "Physical mouse scroll down requested.",
		}, nil
	}
	if strings.Contains(goalLower, "scroll up") {
		return &protocol.AgentAction{
			Action:          protocol.ActionScroll,
			ScrollDirection: "up",
			ScrollAmount:    5,
			Speech:          "Scrolling up.",
			Thought:         "Physical mouse scroll up requested.",
		}, nil
	}

	// Typing Text
	typeRegex := regexp.MustCompile(`(?i)^(?:type|type text|write|input)\s+["']?(.+?)["']?$`)
	if matches := typeRegex.FindStringSubmatch(strings.TrimSpace(goal)); len(matches) > 1 {
		rawText := strings.TrimSpace(matches[1])
		return &protocol.AgentAction{
			Action:  protocol.ActionTypeText,
			Text:    rawText,
			Speech:  fmt.Sprintf("Typing: %s", rawText),
			Thought: fmt.Sprintf("Typing text '%s' via CoreGraphics.", rawText),
		}, nil
	}

	// Key Shortcuts
	keyRegex := regexp.MustCompile(`(?i)^(?:press|hit|send key)\s+(.+)$`)
	if matches := keyRegex.FindStringSubmatch(goalLower); len(matches) > 1 {
		keyStr := strings.TrimSpace(matches[1])
		return &protocol.AgentAction{
			Action:  protocol.ActionKey,
			Key:     keyStr,
			Speech:  fmt.Sprintf("Pressing %s.", keyStr),
			Thought: fmt.Sprintf("Executing key combo '%s'.", keyStr),
		}, nil
	}

	// List Open Windows
	if strings.Contains(goalLower, "list windows") || strings.Contains(goalLower, "open windows") || strings.Contains(goalLower, "show windows") {
		return &protocol.AgentAction{
			Action:  protocol.ActionRunCommand,
			Command: "leo-input windows 2>/dev/null || macpilot-input windows",
			Speech:  "Checking active desktop windows.",
			Thought: "Querying on-screen window inventory.",
		}, nil
	}

	// System status / Battery
	if strings.Contains(goalLower, "battery") || strings.Contains(goalLower, "system status") || strings.Contains(goalLower, "macbook status") {
		return &protocol.AgentAction{
			Action:  protocol.ActionSystemStatus,
			Speech:  "Checking MacBook status and battery.",
			Thought: "System status check requested.",
		}, nil
	}

	// Screen Vision Perception
	if strings.Contains(goalLower, "look at my screen") || strings.Contains(goalLower, "what is on my screen") ||
		strings.Contains(goalLower, "what's on my screen") || strings.Contains(goalLower, "see my screen") ||
		strings.Contains(goalLower, "what do you see") || strings.Contains(goalLower, "can you see") ||
		strings.Contains(goalLower, "analyze screen") || strings.Contains(goalLower, "see macbook screen") ||
		strings.Contains(goalLower, "describe screen") || strings.Contains(goalLower, "read screen") ||
		strings.Contains(goalLower, "tell me what you see") || strings.Contains(goalLower, "what am i looking at") ||
		strings.Contains(goalLower, "what is on screen") {
		return &protocol.AgentAction{
			Action:  protocol.ActionInspectScreen,
			Speech:  "Inspecting your screen now.",
			Thought: "Real-time computer vision analysis requested.",
		}, nil
	}

	// Screenshot
	if strings.Contains(goalLower, "screenshot") || strings.Contains(goalLower, "capture screen") {
		return &protocol.AgentAction{
			Action:  protocol.ActionTakeScreenshot,
			Speech:  "Capturing screen.",
			Thought: "Screen capture requested.",
		}, nil
	}

	// Media Playback
	if strings.Contains(goalLower, "play") || strings.Contains(goalLower, "song") || strings.Contains(goalLower, "youtube") {
		query := goalLower
		for _, prefix := range []string{"open chrome and play", "play", "listen to", "search for"} {
			if strings.HasPrefix(query, prefix) {
				query = strings.TrimSpace(strings.TrimPrefix(query, prefix))
			}
		}
		for _, suffix := range []string{"in chrome", "on youtube", "song"} {
			if strings.HasSuffix(query, suffix) {
				query = strings.TrimSpace(strings.TrimSuffix(query, suffix))
			}
		}
		if strings.Contains(query, "weekend") || strings.Contains(query, "weeknd") {
			query = "the weeknd blinding lights"
		}
		encoded := url.QueryEscape(query)
		cmd := fmt.Sprintf(`open -a "Google Chrome" "https://www.youtube.com/results?search_query=%s" 2>/dev/null || open "https://www.youtube.com/results?search_query=%s"`, encoded, encoded)
		return &protocol.AgentAction{
			Action:  protocol.ActionRunCommand,
			Command: cmd,
			Speech:  fmt.Sprintf("Playing %s on YouTube.", query),
			Thought: fmt.Sprintf("Media playback requested for '%s'.", query),
		}, nil
	}

	// Audio & Volume
	if strings.Contains(goalLower, "volume") || strings.Contains(goalLower, "mute") {
		volCmd := "osascript -e 'set volume output muted true'"
		speech := "Audio muted."
		if !strings.Contains(goalLower, "mute") || strings.Contains(goalLower, "unmute") {
			volCmd = "osascript -e 'set volume output volume 50'"
			speech = "Volume set to 50 percent."
		}
		return &protocol.AgentAction{
			Action:  protocol.ActionRunCommand,
			Command: volCmd,
			Speech:  speech,
			Thought: "Adjusting macOS system volume via AppleScript.",
		}, nil
	}

	// App Launching (e.g. "open chrome", "launch safari", "open terminal")
	appMatch := regexp.MustCompile(`(?i)^(?:open|launch|start|switch to)\s+([a-zA-Z0-9\s]+?)(?:\s+(?:app|application))?$`)
	if matches := appMatch.FindStringSubmatch(goalLower); len(matches) > 1 {
		rawApp := strings.TrimSpace(matches[1])
		appName := rawApp
		switch strings.ToLower(rawApp) {
		case "chrome", "google chrome":
			appName = "Google Chrome"
		case "safari":
			appName = "Safari"
		case "terminal":
			appName = "Terminal"
		case "calculator":
			appName = "Calculator"
		case "notes":
			appName = "Notes"
		case "finder":
			appName = "Finder"
		case "spotify":
			appName = "Spotify"
		case "code", "vscode", "vs code":
			appName = "Visual Studio Code"
		default:
			appName = strings.Title(rawApp)
		}

		return &protocol.AgentAction{
			Action:  protocol.ActionLaunchApp,
			AppName: appName,
			Speech:  fmt.Sprintf("Opening %s.", appName),
			Thought: fmt.Sprintf("Launching application '%s'.", appName),
		}, nil
	}

	return &protocol.AgentAction{
		Action:  protocol.ActionTaskComplete,
		Speech:  fmt.Sprintf("Understood. I've finished processing '%s'.", goal),
		Thought: fmt.Sprintf("Completed evaluation for goal: '%s'", goal),
	}, nil
}

func (p *MetaMuseProvider) callTalkModel(ctx context.Context, goal string) (*protocol.AgentAction, error) {
	systemPrompt := `You are L.E.O., an articulate, conversational, and ultra-capable AI assistant for macOS.
You have a natural human voice and complete control over this MacBook and a dedicated Ubuntu ARM64 virtual computer.
Talk naturally, conversationally, and concisely as L.E.O.
Never say "sir", "all systems online", or "your copilot".
Answer directly in a helpful, friendly, and articulate tone.`

	payload := map[string]interface{}{
		"model": p.TalkModel,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": goal},
		},
		"temperature": 0.3,
		"max_tokens":  1000,
	}

	bodyBytes, _ := json.Marshal(payload)
	log.Printf("🗣️ Calling Meta Talk Model (%s) for conversational speech...", p.TalkModel)
	req, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("talk model error %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &chatResp); err == nil && len(chatResp.Choices) > 0 {
		text := strings.TrimSpace(chatResp.Choices[0].Message.Content)
		if text != "" {
			return &protocol.AgentAction{
				Action:  protocol.ActionTaskComplete,
				Speech:  text,
				Thought: fmt.Sprintf("Spoken conversational reply from %s", p.TalkModel),
			}, nil
		}
	}
	return nil, fmt.Errorf("empty response from talk model")
}

func (p *MetaMuseProvider) synthesizeSpokenSummary(ctx context.Context, goal, toolOutput string) (string, error) {
	systemPrompt := `You are L.E.O., an articulate, conversational AI assistant on macOS.
Speak naturally in first person. Never say "sir", "all systems online", or "your copilot".
Summarize the observed tool output for the user's goal in 1 to 2 clear, helpful, conversational sentences spoken aloud.`

	prompt := fmt.Sprintf("Goal: %s\nObserved Output:\n%s\n\nExplain what happened conversationally in 1-2 friendly sentences:", goal, toolOutput)

	payload := map[string]interface{}{
		"model": p.TalkModel,
		"messages": []map[string]interface{}{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": prompt},
		},
		"temperature": 0.3,
		"max_tokens":  1000,
	}

	bodyBytes, _ := json.Marshal(payload)
	req, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil || resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("synthesize error %d", resp.StatusCode)
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(respBytes, &chatResp); err == nil && len(chatResp.Choices) > 0 {
		return strings.TrimSpace(chatResp.Choices[0].Message.Content), nil
	}
	return "", fmt.Errorf("empty synthesis")
}

func (p *MetaMuseProvider) callControlModel(ctx context.Context, goal string, history []protocol.TaskEvent) (*protocol.AgentAction, error) {
	systemPrompt := `You are L.E.O., the autonomous OS and virtual computer control brain for macOS.
You have direct control over this MacBook and a dedicated Ubuntu ARM64 virtual computer.
Decide the optimal action, coordinates, or shell/API command to achieve the user's goal.
Do not use robotic greetings, and never say "sir", "all systems online", or "your copilot".
For general conversation, choose action "speak" with your conversational answer in "speech".
For computer or macOS tasks, select the best action ("system_status", "launch_app", "web_search", "run_command", "computer_execute_command", etc.).

You can perform the following actions:
1. "click": Left-click at screen coordinate. Use "x": <0-1280>, "y": <0-800>.
2. "double_click": Double-click at screen coordinate. Use "x": <0-1280>, "y": <0-800>.
3. "right_click": Right-click at screen coordinate. Use "x": <0-1280>, "y": <0-800>.
4. "mouse_move": Move cursor to coordinate. Use "x": <0-1280>, "y": <0-800>.
5. "mouse_drag": Click and drag across screen. Use "start_x", "start_y", "x", "y".
6. "scroll": Scroll page. Use "scroll_direction": "up"|"down", "scroll_amount": <1-10>.
7. "key": Press key or shortcut (e.g. "cmd+t", "cmd+w", "cmd+c", "cmd+v", "return", "space", "escape"). Use "key": "<combo>".
8. "type_text": Type text into the focused field. Use "text": "<text to type>".
9. "web_search": Search the live internet in real time. Use "query": "<search keywords>". Use this whenever the user asks for facts, news, documentation, research, recommendations, or current information!
10. "browse_url": Open any website or search in Chrome / Safari. Use "url": "<target url>".
11. "run_command": Run shell or AppleScript commands on macOS (e.g. file operations, terminal tasks, curl, git, osascript).
12. "launch_app": Launch or activate any Mac application (Chrome, Safari, Notes, Terminal, VSCode, Slack, Calculator, Spotify, etc.). Use "app_name": "<name>".
13. "find_files": Search for any file on this Mac using Spotlight. Use "query": "<filename or pattern>".
14. "system_status": Inspect Mac battery, frontmost app, and system volume.
15. "take_screenshot": Capture the MacBook screen.
16. "speak": Directly converse, answer, or explain something to the user with a conversational response.
17. "computer_start": Boot up the dedicated Ubuntu ARM64 virtual computer.
18. "computer_stop": Safely power down the virtual computer while preserving all storage.
19. "computer_status": Check if the virtual computer and guest agent are running.
20. "computer_execute_command": Run bash terminal command inside the Ubuntu virtual computer. Use "command": "<cmd>".
21. "computer_read_file": Read file content inside the virtual computer. Use "path": "<path>".
22. "computer_write_file": Write file inside the virtual computer. Use "path": "<path>", "content": "<data>".
23. "computer_list_files": List files in a directory inside the virtual computer. Use "path": "<dir>".
24. "computer_open_application": Launch application inside the virtual computer (e.g. "chromium-browser", "xfce4-terminal"). Use "app_name": "<name>".
25. "computer_capture_screenshot": Capture screenshot of the virtual computer desktop.
26. "computer_get_metrics": Retrieve CPU, RAM, and Disk metrics of the virtual computer.
27. "task_complete": Conclude the task with a polished, concise verbal summary of what was accomplished, spoken aloud.

Always respond in valid JSON with schema:
{
  "action": "click" | "double_click" | "right_click" | "mouse_move" | "mouse_drag" | "scroll" | "key" | "type_text" | "web_search" | "browse_url" | "run_command" | "launch_app" | "find_files" | "system_status" | "take_screenshot" | "computer_start" | "computer_stop" | "computer_status" | "computer_execute_command" | "computer_read_file" | "computer_write_file" | "computer_list_files" | "computer_open_application" | "computer_capture_screenshot" | "computer_get_metrics" | "speak" | "task_complete",
  "speech": "<Polished, articulate English spoken aloud in real time to the user, in true L.E.O. style>",
  "query": "<query for web_search or find_files>",
  "url": "<url for browse_url>",
  "command": "<terminal / osascript command>",
  "app_name": "<app to launch>",
  "path": "<file or screenshot path>",
  "x": <x coordinate 0-1280>,
  "y": <y coordinate 0-800>,
  "start_x": <start x coordinate>,
  "start_y": <start y coordinate>,
  "scroll_direction": "up" | "down",
  "scroll_amount": <number>,
  "key": "<key combo>",
  "text": "<text to type>",
  "thought": "<concise reasoning>"
}`

	messages := []map[string]interface{}{
		{
			"role":    "system",
			"content": systemPrompt,
		},
		{
			"role":    "user",
			"content": goal,
		},
	}

	for _, h := range history {
		if h.Action != nil {
			actJSON, _ := json.Marshal(h.Action)
			messages = append(messages, map[string]interface{}{
				"role":    "assistant",
				"content": string(actJSON),
			})
			toolResult := h.Message
			if toolResult == "" {
				toolResult = "Action executed successfully."
			}
			messages = append(messages, map[string]interface{}{
				"role":    "user",
				"content": fmt.Sprintf("OBSERVED TOOL OUTPUT for Step %d:\n%s\n\nIf this satisfies the goal, conclude now with action 'task_complete' and explain to the user in 'speech' what was done. Otherwise, decide the next action.", h.Step, toolResult),
			})
		}
	}

	payload := map[string]interface{}{
		"model":       p.ControlModel,
		"messages":    messages,
		"temperature": 0.2,
		"max_tokens":  1500,
	}

	bodyBytes, _ := json.Marshal(payload)
	log.Printf("🕹️ Calling Meta Control Model (%s) for goal: '%s'...", p.ControlModel, goal)
	req, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint, bytes.NewBuffer(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("create request failed: %w", err)
	}

	req.Header.Set("Authorization", "Bearer "+p.APIKey)
	req.Header.Set("Content-Type", "application/json")
	resp, err := p.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response failed: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("control model status %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBytes, &chatResp); err != nil || len(chatResp.Choices) == 0 {
		return nil, fmt.Errorf("invalid response json or empty choices: %w", err)
	}

	content := chatResp.Choices[0].Message.Content
	startIdx := strings.Index(content, "{")
	endIdx := strings.LastIndex(content, "}")
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		jsonStr := content[startIdx : endIdx+1]
		var action protocol.AgentAction
		if json.Unmarshal([]byte(jsonStr), &action) == nil && action.Action != "" {
			if action.Speech == "" && action.Thought != "" {
				action.Speech = action.Thought
			}
			log.Printf("🕹️ Meta Control Model returned action '%s' with speech: '%s'", action.Action, action.Speech)
			return &action, nil
		}
	}

	trimmed := strings.TrimSpace(content)
	if trimmed != "" {
		log.Printf("🕹️ Meta Control Model returned text: '%s'", trimmed)
		return &protocol.AgentAction{
			Action:  protocol.ActionTaskComplete,
			Speech:  trimmed,
			Thought: "Direct response from Meta Control Model.",
		}, nil
	}

	return nil, fmt.Errorf("could not parse action from control model: %s", content)
}

func (p *MetaMuseProvider) summarizeScreenObservation(obs string) string {
	var frontApp, winTitle, browserContent, apps, ocr string
	lines := strings.Split(obs, "\n")
	for _, l := range lines {
		l = strings.TrimSpace(l)
		if strings.HasPrefix(l, "Frontmost Application:") {
			frontApp = strings.TrimSpace(strings.TrimPrefix(l, "Frontmost Application:"))
		} else if strings.HasPrefix(l, "Active Window:") {
			winTitle = strings.TrimSpace(strings.TrimPrefix(l, "Active Window:"))
		} else if strings.HasPrefix(l, "Browser Content:") {
			browserContent = strings.TrimSpace(strings.TrimPrefix(l, "Browser Content:"))
		} else if strings.HasPrefix(l, "Open Applications:") {
			apps = strings.TrimSpace(strings.TrimPrefix(l, "Open Applications:"))
		} else if strings.HasPrefix(l, "Visible Screen Text:") {
			ocr = strings.TrimSpace(strings.TrimPrefix(l, "Visible Screen Text:"))
		}
	}
	_ = apps
	_ = ocr

	if frontApp == "" {
		frontApp = "your desktop"
	}

	// 1. YouTube / Media playback
	if strings.Contains(strings.ToLower(browserContent), "youtube") || strings.Contains(strings.ToLower(winTitle), "youtube") {
		title := winTitle
		if title == "" {
			title = browserContent
		}
		title = strings.TrimSuffix(title, " - YouTube")
		return fmt.Sprintf("I can see your screen right now. You're in Google Chrome watching %s on YouTube.", title)
	}

	// 2. Browser with active tab
	if browserContent != "" {
		return fmt.Sprintf("I'm looking at your screen. You're browsing in %s, currently viewing %s.", frontApp, browserContent)
	}

	// 3. Coding (VS Code / Xcode / Cursor)
	lowerApp := strings.ToLower(frontApp)
	if strings.Contains(lowerApp, "code") || strings.Contains(lowerApp, "xcode") || strings.Contains(lowerApp, "cursor") {
		if winTitle != "" {
			return fmt.Sprintf("You're in %s working on %s. I can see your editor and terminal are active.", frontApp, winTitle)
		}
		return fmt.Sprintf("You're currently working in %s with your codebase open.", frontApp)
	}

	// 4. Terminal / Shell
	if strings.Contains(lowerApp, "terminal") || strings.Contains(lowerApp, "iterm") || strings.Contains(lowerApp, "warp") {
		return fmt.Sprintf("You're in %s running terminal commands.", frontApp)
	}

	// 5. Default natural summary
	if winTitle != "" {
		return fmt.Sprintf("I'm looking at your screen. Your active window is %s in %s.", winTitle, frontApp)
	}
	return fmt.Sprintf("I can see your screen. You're currently using %s.", frontApp)
}

func (p *MetaMuseProvider) AnalyzeScreenVision(ctx context.Context, imageB64, activeApp, winTitle, ocrText, prompt string) (string, error) {
	// Synthesize visual response using Talk Model (muse-spark-1.3)
	if p.APIKey != "" {
		systemPrompt := `You are L.E.O., an articulate, conversational, and observant AI assistant with real-time Computer Vision on macOS.
You are looking directly at the user's MacBook display.
Describe what the user is working on or looking at conversationally and concisely in 1 to 3 spoken sentences.
Answer directly as L.E.O. Never say "sir", "all systems online", or "your copilot".`

		userMsg := fmt.Sprintf("Frontmost Application: %s\nActive Window: %s\nVisible Screen Text & Elements:\n%s\n\nUser Question: %s\n\nExplain what is on screen conversationally in 1-2 friendly spoken sentences:",
			activeApp, winTitle, ocrText, prompt)

		payload := map[string]interface{}{
			"model": p.TalkModel,
			"messages": []map[string]interface{}{
				{"role": "system", "content": systemPrompt},
				{"role": "user", "content": userMsg},
			},
			"temperature": 0.3,
			"max_tokens":  500,
		}

		bodyBytes, _ := json.Marshal(payload)
		req, err := http.NewRequestWithContext(ctx, "POST", p.Endpoint, bytes.NewBuffer(bodyBytes))
		if err == nil {
			req.Header.Set("Authorization", "Bearer "+p.APIKey)
			req.Header.Set("Content-Type", "application/json")
			resp, err := p.HTTPClient.Do(req)
			if err == nil {
				defer resp.Body.Close()
				respBytes, _ := io.ReadAll(resp.Body)
				var chatResp struct {
					Choices []struct {
						Message struct {
							Content string `json:"content"`
						} `json:"message"`
					} `json:"choices"`
				}
				if json.Unmarshal(respBytes, &chatResp) == nil && len(chatResp.Choices) > 0 {
					spoken := strings.TrimSpace(chatResp.Choices[0].Message.Content)
					if spoken != "" {
						return spoken, nil
					}
				}
			}
		}
	}

	// Offline fallback
	obs := fmt.Sprintf("Frontmost Application: %s\nActive Window: %s\nVisible Screen Text:\n%s", activeApp, winTitle, ocrText)
	return p.summarizeScreenObservation(obs), nil
}
