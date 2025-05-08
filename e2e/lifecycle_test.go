package e2e

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/exec"
	"syscall"
	"testing"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/health/grpc_health_v1"
)

const (
	serverHost = "localhost"
	serverPort = 8080
	grpcAddr   = "localhost:8080"
	probesPort = 5555
)

func waitForPort(address string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.Dial("tcp", address)
		if err == nil {
			_ = conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("timeout reached waiting for %s", address)
}

func waitForProbe(url string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	var lastErr error
	for time.Now().Before(deadline) {
		resp, err := http.Get(url)
		if err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return nil
			}
			lastErr = fmt.Errorf("status: %d", resp.StatusCode)
		} else {
			lastErr = err
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("probe %s not ready: %v", url, lastErr)
}

func TestLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, "make", "-f", "Makefile", "run-docker")
	cmd.Dir = ".." // Set the working directory to the parent directory (project root)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		t.Fatalf("failed to start server: %v", err)
	}

	if err := waitForPort(fmt.Sprintf("%s:%d", serverHost, probesPort), 20*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("server probe port didn't open: %v", err)
	}

	startupURL := fmt.Sprintf("http://%s:%d/startup", serverHost, probesPort)
	readinessURL := fmt.Sprintf("http://%s:%d/readiness", serverHost, probesPort)

	if err := waitForProbe(startupURL, 10*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("startup probe failed: %v", err)
	}
	if err := waitForProbe(readinessURL, 10*time.Second); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("readiness probe failed: %v", err)
	}

	conn, err := grpc.Dial(grpcAddr, grpc.WithInsecure(), grpc.WithBlock(), grpc.WithTimeout(5*time.Second))
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("failed to connect to gRPC server: %v", err)
	}
	defer conn.Close()

	healthClient := grpc_health_v1.NewHealthClient(conn)
	resp, err := healthClient.Check(ctx, &grpc_health_v1.HealthCheckRequest{})
	if err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("health check failed: %v", err)
	}
	if resp.GetStatus() != grpc_health_v1.HealthCheckResponse_SERVING {
		_ = cmd.Process.Kill()
		t.Fatalf("server health not serving: %v", resp.GetStatus())
	}

	done := make(chan error)
	go func() {
		done <- cmd.Wait()
	}()

	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		_ = cmd.Process.Kill()
		t.Fatalf("failed to send signal: %v", err)
	}

	select {
	case <-done:
	case <-time.After(10 * time.Second):
		_ = cmd.Process.Kill()
		t.Fatalf("server didn't exit in time after signal")
	}

	// Verify that the server port is closed after shutdown
	err = waitForPort(fmt.Sprintf("%s:%d", serverHost, serverPort), 2*time.Second)
	if err == nil {
		t.Fatalf("server port is still open after shutdown")
	}
}
