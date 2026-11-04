package computer

import (
	"bytes"
	"context"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/macpilot/agent-engine/internal/search"
	"github.com/macpilot/agent-engine/pkg/protocol"
)

type Controller struct {
	searchEngine *search.Engine
}

func NewController() *Controller {
	return &Controller{
		searchEngine: search.NewEngine(),
	}
}

func (c *Controller) Execute(ctx context.Context, action *protocol.AgentAction) (string, error) {
	switch action.Action {
	case protocol.ActionClick:
		x, y := 640, 400
		if action.X != nil && action.Y != nil {
			x, y = *action.X, *action.Y
		}
		return c.Click(ctx, x, y)

	case protocol.ActionDoubleClick:
		x, y := 640, 400
		if action.X != nil && action.Y != nil {
			x, y = *action.X, *action.Y
		}
		return c.DoubleClick(ctx, x, y)

	case protocol.ActionTripleClick:
		x, y := 640, 400
		if action.X != nil && action.Y != nil {
			x, y = *action.X, *action.Y
		}
		return c.TripleClick(ctx, x, y)

	case protocol.ActionRightClick:
		x, y := 640, 400
		if action.X != nil && action.Y != nil {
			x, y = *action.X, *action.Y
		}
		return c.RightClick(ctx, x, y)

	case protocol.ActionMiddleClick:
		x, y := 640, 400
		if action.X != nil && action.Y != nil {
			x, y = *action.X, *action.Y
		}
		return c.MiddleClick(ctx, x, y)

	case protocol.ActionMouseMove:
		x, y := 640, 400
		if action.X != nil && action.Y != nil {
			x, y = *action.X, *action.Y
		}
		return c.MouseMove(ctx, x, y)

	case protocol.ActionMouseDrag:
		sx, sy, ex, ey := 100, 100, 200, 200
		if action.StartX != nil { sx = *action.StartX }
		if action.StartY != nil { sy = *action.StartY }
		if action.X != nil { ex = *action.X }
		if action.Y != nil { ey = *action.Y }
		return c.MouseDrag(ctx, sx, sy, ex, ey)

	case protocol.ActionScroll:
		dir := action.ScrollDirection
		if dir == "" { dir = "down" }
		amt := action.ScrollAmount
		if amt <= 0 { amt = 3 }
		return c.Scroll(ctx, dir, amt)

	case protocol.ActionKey:
		combo := action.Key
		if combo == "" { combo = action.Hotkey }
		return c.PressKey(ctx, combo)

	case protocol.ActionWebSearch:
		query := action.Query
		if query == "" {
			query = action.Command
		}
		if query == "" {
			query = action.Text
		}
		return c.WebSearch(ctx, query)

	case protocol.ActionBrowseURL:
		targetURL := action.URL
		if targetURL == "" {
			targetURL = action.Command
		}
		return c.BrowseURL(ctx, targetURL)

	case protocol.ActionFindFiles:
		return c.FindFiles(ctx, action.Query)

	case protocol.ActionSystemStatus:
		return c.GetSystemStatus(ctx)

	case protocol.ActionTakeScreenshot:
		return c.TakeScreenshot(ctx, action.Path)

	case protocol.ActionInspectScreen:
		return c.InspectScreen(ctx)

	case protocol.ActionRunCommand:
		return c.RunShell(ctx, action.Command)

	case protocol.ActionLaunchApp:
		return c.LaunchApp(ctx, action.AppName)

	case protocol.ActionPressHotkey:
		return c.PressHotkey(ctx, action.Hotkey)

	case protocol.ActionTypeText:
		return c.TypeText(ctx, action.Text)

	case protocol.ActionWait:
		duration := time.Duration(action.Seconds * float64(time.Second))
		if duration <= 0 {
			duration = 1 * time.Second
		}
		time.Sleep(duration)
		return fmt.Sprintf("Waited %.1fs", action.Seconds), nil

	case protocol.ActionTaskComplete:
		return action.Thought, nil

	default:
		return fmt.Sprintf("Action '%s' completed", action.Action), nil
	}
}

func (c *Controller) WebSearch(ctx context.Context, query string) (string, error) {
	if query == "" {
		return "Empty search query", nil
	}
	results, err := c.searchEngine.Search(ctx, query, 5)
	if err != nil {
		return "", fmt.Errorf("web search error: %w", err)
	}
	return search.FormatForLLM(results), nil
}

func (c *Controller) BrowseURL(ctx context.Context, targetURL string) (string, error) {
	if targetURL == "" {
		targetURL = "https://www.google.com"
	}
	cmd := fmt.Sprintf(`open -a "Google Chrome" "%s" 2>/dev/null || open "%s"`, targetURL, targetURL)
	return c.RunShell(ctx, cmd)
}

func (c *Controller) FindFiles(ctx context.Context, query string) (string, error) {
	if query == "" {
		return "No search query provided", nil
	}
	cmd := fmt.Sprintf(`mdfind -name "%s" | head -n 10`, query)
	out, err := c.RunShell(ctx, cmd)
	if err != nil || strings.TrimSpace(out) == "" {
		return fmt.Sprintf("No files found matching '%s'", query), nil
	}
	return out, nil
}

func (c *Controller) GetSystemStatus(ctx context.Context) (string, error) {
	script := `tell application "System Events" to get name of first application process whose frontmost is true`
	frontApp, _ := c.RunAppleScript(ctx, script)
	if frontApp == "" {
		frontApp = "Finder"
	}
	battOut, _ := c.RunShell(ctx, "pmset -g batt | grep -o '[0-9]*%' | head -n 1")
	volOut, _ := c.RunAppleScript(ctx, "output volume of (get volume settings)")
	return fmt.Sprintf("Frontmost App: %s | Battery: %s | Volume: %s%%", frontApp, strings.TrimSpace(battOut), strings.TrimSpace(volOut)), nil
}

var (
	lastGlobalShotTime time.Time
	lastGlobalShotMu   sync.Mutex
)

func (c *Controller) TakeScreenshot(ctx context.Context, path string) (string, error) {
	if path == "" {
		path = fmt.Sprintf("%s/Desktop/macpilot_screenshot_%d.png", os.Getenv("HOME"), time.Now().Unix())
	}
	lastGlobalShotMu.Lock()
	now := time.Now()
	shouldCapture := now.Sub(lastGlobalShotTime) > 8*time.Second
	if shouldCapture {
		lastGlobalShotTime = now
	}
	lastGlobalShotMu.Unlock()

	if shouldCapture {
		cmd := fmt.Sprintf("/usr/sbin/screencapture -x %s", path)
		_, err := c.RunShell(ctx, cmd)
		if err != nil {
			return "", err
		}
	}
	return fmt.Sprintf("Screenshot captured to: %s", path), nil
}

func (c *Controller) InspectScreen(ctx context.Context) (string, error) {
	// 1. Capture screen to temp image (rate-limited to prevent indicator flashing)
	shotPath := "/tmp/macpilot_vision.png"
	lastGlobalShotMu.Lock()
	now := time.Now()
	shouldCapture := now.Sub(lastGlobalShotTime) > 8*time.Second
	if shouldCapture {
		lastGlobalShotTime = now
	}
	lastGlobalShotMu.Unlock()

	if shouldCapture {
		_, _ = c.RunShell(ctx, fmt.Sprintf("/usr/sbin/screencapture -x %s 2>/dev/null || true", shotPath))
	}

	// 2. Frontmost application
	frontApp, _ := c.RunAppleScript(ctx, `tell application "System Events" to get name of first application process whose frontmost is true`)
	if frontApp == "" {
		frontApp = "Desktop / Finder"
	}

	// 3. Frontmost window title
	winTitle, _ := c.RunAppleScript(ctx, `tell application "System Events"
		try
			set frontApp to first application process whose frontmost is true
			return name of front window of frontApp
		on error
			return ""
		end try
	end tell`)

	// 4. Browser active tab context (Chrome & Safari)
	browserInfo := ""
	if strings.Contains(strings.ToLower(frontApp), "chrome") {
		tabInfo, _ := c.RunAppleScript(ctx, `tell application "Google Chrome"
			if (count of windows) > 0 then
				set activeTab to active tab of front window
				return (title of activeTab) & " (" & (URL of activeTab) & ")"
			end if
		end tell`)
		if tabInfo != "" {
			browserInfo = fmt.Sprintf("Chrome Active Tab: %s", tabInfo)
		}
	} else if strings.Contains(strings.ToLower(frontApp), "safari") {
		tabInfo, _ := c.RunAppleScript(ctx, `tell application "Safari"
			if (count of windows) > 0 then
				set activeTab to current tab of front window
				return (name of activeTab) & " (" & (URL of activeTab) & ")"
			end if
		end tell`)
		if tabInfo != "" {
			browserInfo = fmt.Sprintf("Safari Active Tab: %s", tabInfo)
		}
	}

	// 5. Visible applications
	apps, _ := c.RunAppleScript(ctx, `tell application "System Events" to return name of every application process whose visible is true`)

	// 6. Apple Neural Engine OCR Text
	ocrText := ""
	if fileExists(shotPath) {
		ocrScript := fmt.Sprintf(`swift -e '
import Vision
import AppKit

if let image = NSImage(contentsOfFile: "%s"),
   let tiff = image.tiffRepresentation,
   let bitmap = NSBitmapImageRep(data: tiff),
   let cgImage = bitmap.cgImage {
    let req = VNRecognizeTextRequest()
    req.recognitionLevel = .fast
    let handler = VNImageRequestHandler(cgImage: cgImage, options: [:])
    try? handler.perform([req])
    let results = req.results ?? []
    for r in results.prefix(15) {
        if let top = r.topCandidates(1).first {
            print(top.string)
        }
    }
}
'`, shotPath)
		out, err := c.RunShell(ctx, ocrScript)
		if err == nil && strings.TrimSpace(out) != "" {
			ocrText = strings.TrimSpace(out)
		}
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Frontmost Application: %s\n", frontApp))
	if winTitle != "" {
		sb.WriteString(fmt.Sprintf("Active Window: %s\n", winTitle))
	}
	if browserInfo != "" {
		sb.WriteString(fmt.Sprintf("Browser Content: %s\n", browserInfo))
	}
	if apps != "" {
		sb.WriteString(fmt.Sprintf("Open Applications: %s\n", apps))
	}
	if ocrText != "" {
		sb.WriteString(fmt.Sprintf("Visible Screen Text:\n%s\n", ocrText))
	}

	return strings.TrimSpace(sb.String()), nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir() && info.Size() > 0
}

func (c *Controller) RunShell(ctx context.Context, cmdStr string) (string, error) {
	cmd := exec.CommandContext(ctx, "zsh", "-c", cmdStr)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	output := strings.TrimSpace(stdout.String())
	if err != nil {
		errOut := strings.TrimSpace(stderr.String())
		if errOut != "" {
			return "", fmt.Errorf("shell failed: %s (%v)", errOut, err)
		}
		return "", err
	}
	if output == "" {
		return "Command executed successfully with exit code 0", nil
	}
	return output, nil
}

func (c *Controller) RunAppleScript(ctx context.Context, script string) (string, error) {
	cmd := exec.CommandContext(ctx, "osascript", "-e", script)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("applescript error: %s", stderr.String())
	}
	return strings.TrimSpace(stdout.String()), nil
}

func (c *Controller) LaunchApp(ctx context.Context, appName string) (string, error) {
	script := fmt.Sprintf(`tell application "%s" to activate`, appName)
	_, err := c.RunAppleScript(ctx, script)
	if err != nil {
		cmd := fmt.Sprintf(`open -a "%s"`, appName)
		return c.RunShell(ctx, cmd)
	}
	return fmt.Sprintf("Application '%s' activated", appName), nil
}

func (c *Controller) PressHotkey(ctx context.Context, hotkey string) (string, error) {
	out, err := c.runInput(ctx, "key", hotkey)
	if err == nil {
		return fmt.Sprintf("Hotkey '%s' simulated (%s)", hotkey, out), nil
	}
	lower := strings.ToLower(hotkey)
	if lower == "enter" || lower == "return" {
		script := `tell application "System Events" to key code 36`
		return c.RunAppleScript(ctx, script)
	} else if lower == "cmd+space" {
		script := `tell application "System Events" to key code 49 using {command down}`
		return c.RunAppleScript(ctx, script)
	}
	return fmt.Sprintf("Hotkey '%s' simulated", hotkey), nil
}

func (c *Controller) TypeText(ctx context.Context, text string) (string, error) {
	out, err := c.runInput(ctx, "type", text)
	if err == nil {
		return fmt.Sprintf("Typed text via CoreGraphics (%s)", out), nil
	}
	escaped := strings.ReplaceAll(text, `"`, `\"`)
	script := fmt.Sprintf(`tell application "System Events" to keystroke "%s"`, escaped)
	return c.RunAppleScript(ctx, script)
}

func (c *Controller) PlayMediaInBrowser(ctx context.Context, query string) (string, error) {
	cleanQuery := strings.TrimSpace(query)
	if cleanQuery == "" {
		cleanQuery = "the weeknd blinding lights"
	}
	encoded := url.QueryEscape(cleanQuery)
	ytURL := fmt.Sprintf("https://www.youtube.com/results?search_query=%s", encoded)
	cmd := fmt.Sprintf(`open -a "Google Chrome" "%s" 2>/dev/null || open "%s"`, ytURL, ytURL)
	return c.RunShell(ctx, cmd)
}

func (c *Controller) OpenFinder(ctx context.Context, targetPath string) (string, error) {
	if targetPath == "" {
		targetPath = "~/Desktop"
	}
	cmd := fmt.Sprintf("open %s", targetPath)
	return c.RunShell(ctx, cmd)
}

func (c *Controller) SetVolume(ctx context.Context, volumePercent int) (string, error) {
	script := fmt.Sprintf("set volume output volume %d", volumePercent)
	return c.RunAppleScript(ctx, script)
}

func getInputBinary() string {
	execPath, _ := os.Executable()
	var execDir string
	if execPath != "" {
		execDir = filepath.Dir(execPath)
	}

	paths := []string{
		filepath.Join(execDir, "leo-input"),
		filepath.Join(execDir, "macpilot-input"),
		filepath.Join(execDir, "..", "Resources", "bin", "leo-input"),
		filepath.Join(execDir, "..", "Resources", "bin", "macpilot-input"),
		"./bin/leo-input",
		"./bin/macpilot-input",
		"services/agent-engine/bin/leo-input",
		"services/agent-engine/bin/macpilot-input",
		"/Users/reyansh/Desktop/contribute/auto agent/MacPilotAI/services/agent-engine/bin/leo-input",
		"/Users/reyansh/Desktop/contribute/auto agent/MacPilotAI/services/agent-engine/bin/macpilot-input",
	}
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}
	return "leo-input"
}

func (c *Controller) runInput(ctx context.Context, args ...string) (string, error) {
	bin := getInputBinary()
	cmd := exec.CommandContext(ctx, bin, args...)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("input control failed: %s (%v)", strings.TrimSpace(string(out)), err)
	}
	return strings.TrimSpace(string(out)), nil
}

func (c *Controller) Click(ctx context.Context, x, y int) (string, error) {
	return c.runInput(ctx, "click", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
}

func (c *Controller) DoubleClick(ctx context.Context, x, y int) (string, error) {
	return c.runInput(ctx, "double_click", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
}

func (c *Controller) TripleClick(ctx context.Context, x, y int) (string, error) {
	return c.runInput(ctx, "triple_click", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
}

func (c *Controller) RightClick(ctx context.Context, x, y int) (string, error) {
	return c.runInput(ctx, "right_click", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
}

func (c *Controller) MiddleClick(ctx context.Context, x, y int) (string, error) {
	return c.runInput(ctx, "middle_click", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
}

func (c *Controller) MouseMove(ctx context.Context, x, y int) (string, error) {
	return c.runInput(ctx, "move", fmt.Sprintf("%d", x), fmt.Sprintf("%d", y))
}

func (c *Controller) MouseDrag(ctx context.Context, sx, sy, ex, ey int) (string, error) {
	return c.runInput(ctx, "drag", fmt.Sprintf("%d", sx), fmt.Sprintf("%d", sy), fmt.Sprintf("%d", ex), fmt.Sprintf("%d", ey))
}

func (c *Controller) Scroll(ctx context.Context, direction string, amount int) (string, error) {
	return c.runInput(ctx, "scroll", direction, fmt.Sprintf("%d", amount))
}

func (c *Controller) PressKey(ctx context.Context, combo string) (string, error) {
	return c.runInput(ctx, "key", combo)
}
