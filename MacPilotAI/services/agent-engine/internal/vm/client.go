package vm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type GuestClient struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewGuestClient(port int, token string) *GuestClient {
	return &GuestClient{
		baseURL: fmt.Sprintf("http://127.0.0.1:%d", port),
		token:   token,
		httpClient: &http.Client{
			Timeout: 60 * time.Second,
		},
	}
}

func (c *GuestClient) newRequest(ctx context.Context, method, endpoint string, body interface{}) (*http.Request, error) {
	var bodyReader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		bodyReader = bytes.NewReader(data)
	}

	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+endpoint, bodyReader)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")
	if c.token != "" {
		req.Header.Set("Authorization", "Bearer "+c.token)
	}
	return req, nil
}

func (c *GuestClient) Health(ctx context.Context) (bool, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/health", nil)
	if err != nil {
		return false, err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	return resp.StatusCode == http.StatusOK, nil
}

type CommandExecResult struct {
	ID         string  `json:"id"`
	ExitCode   int     `json:"exit_code"`
	Stdout     string  `json:"stdout"`
	Stderr     string  `json:"stderr"`
	ElapsedSec float64 `json:"elapsed_sec"`
	Success    bool    `json:"success"`
}

func (c *GuestClient) ExecCommand(ctx context.Context, command string, timeoutSec int, workDir string) (*CommandExecResult, error) {
	payload := map[string]interface{}{
		"command":     command,
		"timeout_sec": timeoutSec,
		"work_dir":    workDir,
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/commands/exec", payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("guest agent exec request failed: %w", err)
	}
	defer resp.Body.Close()

	var result CommandExecResult
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode exec result: %w", err)
	}
	return &result, nil
}

func (c *GuestClient) CancelCommand(ctx context.Context, cmdID string) error {
	payload := map[string]string{"id": cmdID}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/commands/cancel", payload)
	if err != nil {
		return err
	}
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	return nil
}

func (c *GuestClient) ReadFile(ctx context.Context, path, encoding string) (string, error) {
	payload := map[string]string{
		"path":     path,
		"encoding": encoding,
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/fs/read", payload)
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Content string `json:"content"`
		Error   string `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.Error != "" {
		return "", fmt.Errorf("%s", result.Error)
	}
	return result.Content, nil
}

func (c *GuestClient) WriteFile(ctx context.Context, path, content, encoding string, mode uint32) error {
	payload := map[string]interface{}{
		"path":     path,
		"content":  content,
		"encoding": encoding,
		"mode":     mode,
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/fs/write", payload)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("write file failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

type FileInfo struct {
	Name    string    `json:"name"`
	Path    string    `json:"path"`
	IsDir   bool      `json:"is_dir"`
	Size    int64     `json:"size"`
	Mode    string    `json:"mode"`
	ModTime time.Time `json:"mod_time"`
}

func (c *GuestClient) ListFiles(ctx context.Context, path string) ([]FileInfo, error) {
	payload := map[string]string{"path": path}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/fs/list", payload)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Entries []FileInfo `json:"entries"`
		Error   string     `json:"error,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	if result.Error != "" {
		return nil, fmt.Errorf("%s", result.Error)
	}
	return result.Entries, nil
}

type AppInfo struct {
	Name        string `json:"name"`
	Exec        string `json:"exec"`
	Comment     string `json:"comment"`
	DesktopFile string `json:"desktop_file"`
}

func (c *GuestClient) ListApps(ctx context.Context) ([]AppInfo, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/apps", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result struct {
		Apps []AppInfo `json:"apps"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result.Apps, nil
}

func (c *GuestClient) LaunchApp(ctx context.Context, name string, args []string) error {
	payload := map[string]interface{}{
		"name": name,
		"args": args,
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/apps/launch", payload)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("launch app failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

type ScreenshotInfo struct {
	ImageBase64 string `json:"image_base64"`
	Format      string `json:"format"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
}

func (c *GuestClient) CaptureScreenshot(ctx context.Context) (*ScreenshotInfo, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/desktop/screenshot", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result ScreenshotInfo
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (c *GuestClient) SendInput(ctx context.Context, action string, x, y int, text, key string) error {
	payload := map[string]interface{}{
		"action": action,
		"x":      x,
		"y":      y,
		"text":   text,
		"key":    key,
	}
	req, err := c.newRequest(ctx, http.MethodPost, "/api/v1/desktop/input", payload)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("input failed (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

func (c *GuestClient) GetMetrics(ctx context.Context) (*ComputerMetrics, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/api/v1/metrics", nil)
	if err != nil {
		return nil, err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var metrics ComputerMetrics
	if err := json.NewDecoder(resp.Body).Decode(&metrics); err != nil {
		return nil, err
	}
	metrics.Timestamp = time.Now().UTC()
	return &metrics, nil
}

