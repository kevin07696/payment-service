package testutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"testing"
	"time"
)

// NgrokTunnel represents an active ngrok tunnel
type NgrokTunnel struct {
	cmd       *exec.Cmd
	url       string
	autoStart bool
}

// URL returns the public ngrok URL
func (n *NgrokTunnel) URL() string {
	return n.url
}

// Close stops the ngrok tunnel if it was auto-started
func (n *NgrokTunnel) Close() {
	if n.autoStart && n.cmd != nil && n.cmd.Process != nil {
		n.cmd.Process.Kill()
	}
}

// StartNgrokIfNeeded returns a callback URL for EPX to reach.
// It checks in this order:
// 1. CALLBACK_BASE_URL environment variable (manual ngrok or staging URL)
// 2. Auto-start ngrok if installed
// 3. Skip test if neither available
func StartNgrokIfNeeded(t *testing.T, port int) *NgrokTunnel {
	t.Helper()

	// Check if CALLBACK_BASE_URL already set (manual ngrok or staging)
	if url := os.Getenv("CALLBACK_BASE_URL"); url != "" {
		t.Logf("📡 Using existing callback URL: %s", url)
		return &NgrokTunnel{
			url:       url,
			autoStart: false,
		}
	}

	// Try to start ngrok automatically
	if !commandExists("ngrok") {
		t.Skip("⏭️  ngrok not installed and CALLBACK_BASE_URL not set - skipping test requiring external callback")
	}

	t.Log("🚀 Auto-starting ngrok tunnel...")
	tunnel := startNgrok(t, port)
	t.Logf("✅ Ngrok tunnel started: %s", tunnel.url)
	t.Logf("🔍 Ngrok dashboard: http://localhost:4040")

	return tunnel
}

// commandExists checks if a command is available in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// startNgrok starts ngrok tunnel and returns public URL
func startNgrok(t *testing.T, port int) *NgrokTunnel {
	t.Helper()

	// Check if ngrok is already running
	if isNgrokRunning() {
		url, err := getNgrokURL()
		if err == nil && url != "" {
			t.Log("⚠️  Using existing ngrok tunnel")
			return &NgrokTunnel{
				url:       url,
				autoStart: false,
			}
		}
	}

	// Start ngrok in background
	cmd := exec.Command("ngrok", "http", fmt.Sprintf("%d", port), "--log=stdout")

	// Redirect ngrok output to test log for debugging
	logFile, err := os.CreateTemp("", "ngrok-*.log")
	if err == nil {
		cmd.Stdout = logFile
		cmd.Stderr = logFile
		t.Logf("📝 Ngrok logs: %s", logFile.Name())
		t.Cleanup(func() {
			logFile.Close()
			os.Remove(logFile.Name())
		})
	}

	err = cmd.Start()
	if err != nil {
		t.Fatalf("❌ Failed to start ngrok: %v", err)
	}

	// Ensure cleanup on test completion
	t.Cleanup(func() {
		if cmd.Process != nil {
			cmd.Process.Kill()
		}
	})

	// Wait for ngrok to initialize
	t.Log("⏳ Waiting for ngrok to initialize...")
	time.Sleep(3 * time.Second)

	// Get public URL from ngrok API (retry up to 10 times)
	var url string
	for i := 0; i < 10; i++ {
		url, err = getNgrokURL()
		if err == nil && url != "" {
			break
		}
		if i < 9 {
			time.Sleep(1 * time.Second)
		}
	}

	if url == "" {
		t.Fatal("❌ Failed to get ngrok public URL after 10 attempts")
	}

	return &NgrokTunnel{
		cmd:       cmd,
		url:       url,
		autoStart: true,
	}
}

// isNgrokRunning checks if ngrok is already running
func isNgrokRunning() bool {
	resp, err := http.Get("http://localhost:4040/api/tunnels")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	return resp.StatusCode == 200
}

// getNgrokURL retrieves the public URL from ngrok API
func getNgrokURL() (string, error) {
	resp, err := http.Get("http://localhost:4040/api/tunnels")
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var result struct {
		Tunnels []struct {
			PublicURL string `json:"public_url"`
			Proto     string `json:"proto"`
		} `json:"tunnels"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}

	// Find HTTPS tunnel (EPX requires HTTPS for callbacks)
	for _, tunnel := range result.Tunnels {
		if tunnel.Proto == "https" {
			return tunnel.PublicURL, nil
		}
	}

	// Fallback to first tunnel if no HTTPS found
	if len(result.Tunnels) > 0 {
		return result.Tunnels[0].PublicURL, nil
	}

	return "", fmt.Errorf("no tunnels found")
}
