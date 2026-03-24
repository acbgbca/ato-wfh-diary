//go:build e2e_docker

package e2e_test

import (
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/go-rod/rod"
	"github.com/go-rod/rod/lib/launcher"
)

// dockerAuthHeader matches the FORWARD_AUTH_HEADER set in docker-compose.test.yml.
const dockerAuthHeader = "X-Forwarded-User"

// dockerTestURL is the base URL of the container exposed by docker-compose.test.yml.
const dockerTestURL = "http://localhost:18080"

// TestMain builds and starts the Docker container before running E2E tests,
// then tears it down afterwards. Using a separate port (18080) avoids
// conflicts with a locally-running production compose stack on 8080.
func TestMain(m *testing.M) {
	os.Exit(runDockerTests(m))
}

func runDockerTests(m *testing.M) int {
	rootDir, err := findProjectRoot()
	if err != nil {
		log.Printf("find project root: %v", err)
		return 1
	}

	composeFile := filepath.Join(rootDir, "docker-compose.test.yml")

	log.Println("e2e_docker: building and starting container...")
	up := exec.Command("docker", "compose", "-f", composeFile, "up", "--build", "-d")
	up.Stdout = os.Stdout
	up.Stderr = os.Stderr
	if err := up.Run(); err != nil {
		log.Printf("docker compose up: %v", err)
		return 1
	}

	defer func() {
		log.Println("e2e_docker: stopping and removing container...")
		down := exec.Command("docker", "compose", "-f", composeFile, "down", "-v")
		down.Stdout = os.Stdout
		down.Stderr = os.Stderr
		down.Run()
	}()

	log.Printf("e2e_docker: waiting for server at %s...", dockerTestURL)
	if err := waitForServer(dockerTestURL, 60*time.Second); err != nil {
		// Dump container logs so the failure is diagnosable without SSH access.
		log.Println("e2e_docker: container logs:")
		dump := exec.Command("docker", "compose", "-f", composeFile, "logs")
		dump.Stdout = os.Stdout
		dump.Stderr = os.Stderr
		dump.Run()
		log.Printf("e2e_docker: server did not become ready: %v", err)
		return 1
	}
	log.Println("e2e_docker: server ready, running tests")

	return m.Run()
}

// waitForServer polls url until it gets any HTTP response or the timeout expires.
// A 401 (no auth header) counts as ready — it means the server is up.
func waitForServer(url string, timeout time.Duration) error {
	client := &http.Client{Timeout: 2 * time.Second}
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("not ready after %v", timeout)
}

// findProjectRoot walks up from the current directory to find the repo root,
// identified by the presence of docker-compose.test.yml.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "docker-compose.test.yml")); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", fmt.Errorf("docker-compose.test.yml not found from %s", dir)
		}
		dir = parent
	}
}

// newE2EServer returns the Docker container's base URL. The container is
// already running (started by TestMain) and shared across all tests.
func newE2EServer(t *testing.T) string {
	t.Helper()
	return dockerTestURL
}

// newPage launches a headless browser page pre-authenticated as a user
// derived from the test name. Using the test name (rather than a shared
// "alice") isolates each test to its own user in the shared Docker database,
// preventing saved entries from one test polluting another.
func newPage(t *testing.T, _ string) (*rod.Browser, *rod.Page) {
	t.Helper()

	l := launcher.New().Headless(true)
	if path, ok := launcher.LookPath(); ok {
		l = l.Bin(path)
	}
	controlURL := l.MustLaunch()

	browser := rod.New().ControlURL(controlURL).MustConnect()
	t.Cleanup(func() { browser.MustClose() })

	page := browser.MustPage("")
	cleanup, err := page.SetExtraHeaders([]string{dockerAuthHeader, t.Name()})
	if err != nil {
		t.Fatalf("set extra headers: %v", err)
	}
	t.Cleanup(cleanup)

	return browser, page
}
