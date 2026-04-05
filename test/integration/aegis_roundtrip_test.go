package integration_test

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"math/rand/v2"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/matstech/aegis-go/client"
)

const (
	defaultAegisImage   = "matteos93/aegis:latest"
	defaultHTTPBinImage = "ghcr.io/mccutchen/go-httpbin:latest"
)

func TestAegisRoundTrip(t *testing.T) {
	if os.Getenv("AEGIS_RUN_INTEGRATION") != "1" {
		t.Skip("set AEGIS_RUN_INTEGRATION=1 to run Docker integration tests")
	}

	if _, err := exec.LookPath("docker"); err != nil {
		t.Fatalf("docker is required: %v", err)
	}

	runID := fmt.Sprintf("%d-%d", time.Now().Unix(), rand.Uint64())
	networkName := "aegis-go-itest-" + runID
	httpbinName := "aegis-go-httpbin-" + runID
	aegisName := "aegis-go-server-" + runID
	httpbinPort := freePort(t)
	aegisPort := freePort(t)
	probesPort := freePort(t)

	configDir := t.TempDir()
	configPath := filepath.Join(configDir, "config.json")
	if err := os.WriteFile(configPath, []byte(fmt.Sprintf(`{
  "ginmode": "release",
  "loglevel": "debug",
  "server": {
    "mode": "PLAIN",
    "port": 8080,
    "probesport": 2112,
    "upstream": "%s:8080",
    "dropHeaders": ["Authorization", "X-Drop-Me"],
    "injectHeaders": [
      {"name": "X-Aegis-Proxy", "value": "true"},
      {"name": "Authorization", "valueFromEnv": "UPSTREAM_AUTHORIZATION"}
    ]
  },
  "kids": ["test"]
}`, httpbinName)), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	docker := dockerRunner{t: t}

	docker.run("network", "create", networkName)
	t.Cleanup(func() {
		docker.runQuiet("rm", "-f", aegisName)
		docker.runQuiet("rm", "-f", httpbinName)
		docker.runQuiet("network", "rm", networkName)
	})

	docker.run(
		"run", "--detach", "--rm",
		"--name", httpbinName,
		"--network", networkName,
		"--publish", fmt.Sprintf("%d:8080", httpbinPort),
		envOrDefault("HTTPBIN_IMAGE", defaultHTTPBinImage),
	)
	waitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%d/status/200", httpbinPort), 30*time.Second)

	docker.run(
		"run", "--detach", "--rm",
		"--name", aegisName,
		"--network", networkName,
		"--publish", fmt.Sprintf("%d:8080", aegisPort),
		"--publish", fmt.Sprintf("%d:2112", probesPort),
		"--volume", configDir+":/etc/config:ro",
		"--env", "CONFIG_PATH=/etc/config/",
		"--env", "ACCESSKEY_TEST=integration-secret",
		"--env", "UPSTREAM_AUTHORIZATION=Bearer upstream-secret",
		envOrDefault("AEGIS_IMAGE", defaultAegisImage),
	)
	waitForHTTP(t, fmt.Sprintf("http://127.0.0.1:%d/readiness", probesPort), 30*time.Second)

	httpClient := &http.Client{
		Timeout: 15 * time.Second,
		Transport: client.NewTransport(nil, client.Config{
			Kid:           "test",
			Secret:        "integration-secret",
			SignedHeaders: []string{"Content-Type", "X-Drop-Me", "Authorization"},
		}),
	}

	req, err := http.NewRequest(
		http.MethodPost,
		fmt.Sprintf("http://127.0.0.1:%d/anything", aegisPort),
		bytes.NewBufferString(`{"message":"integration-test"}`),
	)
	if err != nil {
		t.Fatalf("http.NewRequest() error = %v", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer client-token")
	req.Header.Set("X-Drop-Me", "drop-me")

	resp, err := httpClient.Do(req)
	if err != nil {
		t.Fatalf("httpClient.Do() error = %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("response status = %d, want %d", resp.StatusCode, http.StatusOK)
	}

	var payload struct {
		Headers map[string][]string `json:"headers"`
		JSON    map[string]any      `json:"json"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if got := firstHeader(payload.Headers, "Authorization"); got != "Bearer upstream-secret" {
		t.Fatalf("Authorization header = %q, want %q", got, "Bearer upstream-secret")
	}
	if got := firstHeader(payload.Headers, "X-Aegis-Proxy"); got != "true" {
		t.Fatalf("X-Aegis-Proxy header = %q, want %q", got, "true")
	}
	if got := firstHeader(payload.Headers, "X-Drop-Me"); got != "" {
		t.Fatalf("X-Drop-Me header = %q, want empty", got)
	}
	if got := firstHeader(payload.Headers, "Auth-Kid"); got != "" {
		t.Fatalf("Auth-Kid header = %q, want empty", got)
	}
	if got := firstHeader(payload.Headers, "Auth-Headers"); got != "" {
		t.Fatalf("Auth-Headers header = %q, want empty", got)
	}
	if got := firstHeader(payload.Headers, "Signature"); got != "" {
		t.Fatalf("Signature header = %q, want empty", got)
	}
	if got := firstHeader(payload.Headers, "Auth-CorrelationId"); got == "" {
		t.Fatal("Auth-CorrelationId header = empty, want propagated value")
	}
	if got := payload.JSON["message"]; got != "integration-test" {
		t.Fatalf("json.message = %#v, want %q", got, "integration-test")
	}
}

type dockerRunner struct {
	t *testing.T
}

func (d dockerRunner) run(args ...string) {
	d.runCommand(false, args...)
}

func (d dockerRunner) runQuiet(args ...string) {
	d.runCommand(true, args...)
}

func (d dockerRunner) runCommand(allowFailure bool, args ...string) {
	d.t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	cmd := exec.CommandContext(ctx, "docker", args...)
	output, err := cmd.CombinedOutput()
	if err != nil && !allowFailure {
		d.t.Fatalf("docker %s failed: %v\n%s", strings.Join(args, " "), err, output)
	}
}

func waitForHTTP(t *testing.T, url string, timeout time.Duration) {
	t.Helper()

	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode < 500 {
				return
			}
		}
		time.Sleep(1 * time.Second)
	}

	t.Fatalf("service did not become ready: %s", url)
}

func freePort(t *testing.T) int {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("net.Listen() error = %v", err)
	}
	defer listener.Close()

	return listener.Addr().(*net.TCPAddr).Port
}

func firstHeader(headers map[string][]string, name string) string {
	for headerName, values := range headers {
		if strings.EqualFold(headerName, name) && len(values) > 0 {
			return values[0]
		}
	}

	return ""
}

func envOrDefault(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
