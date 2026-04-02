/*
 * Licensed to the Apache Software Foundation (ASF) under one or more
 * contributor license agreements.  See the NOTICE file distributed with
 * this work for additional information regarding copyright ownership.
 * The ASF licenses this file to You under the Apache License, Version 2.0
 * (the "License"); you may not use this file except in compliance with
 * the License.  You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

package installer

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	seatunnelmeta "github.com/seatunnel/seatunnelX/internal/seatunnel"
)

const seatunnelxJavaProxyStopTimeout = 8 * time.Second

// SeatunnelXJavaProxyServiceStatus describes the current managed seatunnelx-java-proxy state.
type SeatunnelXJavaProxyServiceStatus struct {
	Service  string `json:"service"`
	Managed  bool   `json:"managed"`
	Running  bool   `json:"running"`
	Healthy  bool   `json:"healthy"`
	Endpoint string `json:"endpoint,omitempty"`
	Port     int    `json:"port,omitempty"`
	PID      int    `json:"pid,omitempty"`
	LogPath  string `json:"log_path,omitempty"`
	StateDir string `json:"state_dir,omitempty"`
	Message  string `json:"message,omitempty"`
}

// StartManagedSeatunnelXJavaProxyService ensures the managed seatunnelx-java-proxy service is available.
func StartManagedSeatunnelXJavaProxyService(ctx context.Context, installDir string, seatunnelVersion string) (*SeatunnelXJavaProxyServiceStatus, error) {
	status, _ := GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir)
	if status != nil && status.Healthy {
		return status, nil
	}

	baseURL, err := ensureSeatunnelXJavaProxyService(ctx, installDir, seatunnelVersion)
	if err != nil {
		return nil, err
	}

	status, statusErr := GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir)
	if statusErr != nil {
		return &SeatunnelXJavaProxyServiceStatus{
			Service:  "seatunnelx_java_proxy",
			Managed:  true,
			Running:  true,
			Healthy:  true,
			Endpoint: baseURL,
			Message:  "seatunnelx-java-proxy service started",
			StateDir: seatunnelxJavaProxyServiceStateDir(installDir),
			LogPath:  filepath.Join(seatunnelxJavaProxyServiceStateDir(installDir), "service.log"),
		}, nil
	}
	status.Message = firstNonBlank(status.Message, "seatunnelx-java-proxy service started")
	return status, nil
}

// GetManagedSeatunnelXJavaProxyServiceStatus returns the current seatunnelx-java-proxy state.
func GetManagedSeatunnelXJavaProxyServiceStatus(ctx context.Context, installDir string) (*SeatunnelXJavaProxyServiceStatus, error) {
	status := &SeatunnelXJavaProxyServiceStatus{
		Service:  "seatunnelx_java_proxy",
		Managed:  true,
		StateDir: seatunnelxJavaProxyServiceStateDir(installDir),
		LogPath:  filepath.Join(seatunnelxJavaProxyServiceStateDir(installDir), "service.log"),
	}

	if endpoint := strings.TrimSpace(os.Getenv(seatunnelxJavaProxyEndpointEnvVar)); endpoint != "" {
		normalized := strings.TrimRight(endpoint, "/")
		status.Managed = false
		status.Endpoint = normalized
		if port := seatunnelxJavaProxyPortFromEndpoint(normalized); port > 0 {
			status.Port = port
		}
		err := waitForSeatunnelXJavaProxyHealthy(ctx, normalized, 1500*time.Millisecond)
		status.Healthy = err == nil
		status.Running = status.Healthy
		if status.Healthy {
			status.Message = "using configured external seatunnelx-java-proxy endpoint"
		} else {
			status.Message = firstNonBlank(seatunnelxJavaProxyErrorString(err), "configured seatunnelx-java-proxy endpoint is unhealthy")
		}
		return status, nil
	}

	if bytes, err := os.ReadFile(filepath.Join(status.StateDir, "service.port")); err == nil {
		if port, ok := parseSeatunnelXJavaProxyPort(strings.TrimSpace(string(bytes))); ok {
			status.Port = port
			status.Endpoint = seatunnelxJavaProxyServiceBaseURL(port)
		}
	}
	if bytes, err := os.ReadFile(filepath.Join(status.StateDir, "service.pid")); err == nil {
		if pid, err := strconv.Atoi(strings.TrimSpace(string(bytes))); err == nil && pid > 0 {
			status.PID = pid
		}
	}

	if status.Endpoint == "" {
		for _, port := range seatunnelxJavaProxyPortCandidates(status.StateDir) {
			if port <= 0 {
				continue
			}
			endpoint := seatunnelxJavaProxyServiceBaseURL(port)
			if err := waitForSeatunnelXJavaProxyHealthy(ctx, endpoint, 1200*time.Millisecond); err == nil {
				status.Endpoint = endpoint
				status.Port = port
				status.Healthy = true
				status.Running = true
				_ = os.MkdirAll(status.StateDir, 0o755)
				_ = os.WriteFile(filepath.Join(status.StateDir, "service.port"), []byte(strconv.Itoa(port)+"\n"), 0o644)
				break
			}
		}
	}

	if status.PID <= 0 && status.Port > 0 {
		if pid := seatunnelxJavaProxyPIDByPort(ctx, status.Port); pid > 0 {
			status.PID = pid
			_ = os.MkdirAll(status.StateDir, 0o755)
			_ = os.WriteFile(filepath.Join(status.StateDir, "service.pid"), []byte(strconv.Itoa(pid)+"\n"), 0o644)
		}
	}
	if status.PID > 0 {
		status.Running = seatunnelxJavaProxyPIDAlive(status.PID)
	}
	if status.Endpoint != "" && !status.Healthy {
		if err := waitForSeatunnelXJavaProxyHealthy(ctx, status.Endpoint, 1500*time.Millisecond); err == nil {
			status.Healthy = true
			status.Running = true
		}
	}

	if status.Managed && strings.TrimSpace(status.LogPath) != "" && status.Running {
		if _, err := os.Stat(status.LogPath); os.IsNotExist(err) {
			_ = os.MkdirAll(filepath.Dir(status.LogPath), 0o755)
			_ = os.WriteFile(status.LogPath, []byte{}, 0o644)
		}
	}

	switch {
	case status.Healthy:
		status.Message = "seatunnelx-java-proxy service is healthy"
	case status.Running:
		status.Message = "seatunnelx-java-proxy service process is running but health check failed"
	default:
		status.Message = "seatunnelx-java-proxy service is not running"
	}

	return status, nil
}

// StopManagedSeatunnelXJavaProxyService stops the locally managed seatunnelx-java-proxy service.
func StopManagedSeatunnelXJavaProxyService(ctx context.Context, installDir string) (*SeatunnelXJavaProxyServiceStatus, error) {
	status, err := GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir)
	if err != nil {
		return nil, err
	}
	if status == nil {
		return nil, fmt.Errorf("seatunnelx-java-proxy service status is unavailable")
	}
	if !status.Managed {
		status.Message = "configured external seatunnelx-java-proxy endpoint cannot be stopped by agent"
		return status, errors.New(status.Message)
	}
	if status.PID <= 0 && !status.Running {
		status.Message = "seatunnelx-java-proxy service is already stopped"
		return status, nil
	}
	if status.PID <= 0 {
		status.Message = "seatunnelx-java-proxy service PID is unavailable; unable to stop managed process safely"
		return status, errors.New(status.Message)
	}

	process, err := os.FindProcess(status.PID)
	if err != nil {
		return status, err
	}
	if err := process.Signal(syscall.SIGTERM); err != nil && !errors.Is(err, os.ErrProcessDone) {
		return status, err
	}
	if waitErr := waitForSeatunnelXJavaProxyShutdown(ctx, status.Endpoint, status.PID, seatunnelxJavaProxyStopTimeout); waitErr != nil {
		_ = process.Signal(syscall.SIGKILL)
		if killWaitErr := waitForSeatunnelXJavaProxyShutdown(ctx, status.Endpoint, status.PID, 3*time.Second); killWaitErr != nil {
			status.Message = "failed to stop seatunnelx-java-proxy service"
			return status, waitErr
		}
	}

	_ = os.Remove(filepath.Join(status.StateDir, "service.pid"))
	stoppedStatus, statusErr := GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir)
	if statusErr != nil {
		return &SeatunnelXJavaProxyServiceStatus{
			Service:  "seatunnelx_java_proxy",
			Managed:  true,
			Running:  false,
			Healthy:  false,
			Endpoint: status.Endpoint,
			Port:     status.Port,
			LogPath:  status.LogPath,
			StateDir: status.StateDir,
			Message:  "seatunnelx-java-proxy service stopped",
		}, nil
	}
	stoppedStatus.Message = "seatunnelx-java-proxy service stopped"
	return stoppedStatus, nil
}

func waitForSeatunnelXJavaProxyShutdown(ctx context.Context, endpoint string, pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		pidAlive := pid > 0 && seatunnelxJavaProxyPIDAlive(pid)
		healthy := false
		if strings.TrimSpace(endpoint) != "" {
			healthy = waitForSeatunnelXJavaProxyHealthy(ctx, endpoint, 500*time.Millisecond) == nil
		}
		if !pidAlive && !healthy {
			return nil
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("timed out waiting for seatunnelx-java-proxy service to stop")
		}
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(250 * time.Millisecond):
		}
	}
}

func seatunnelxJavaProxyPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	process, err := os.FindProcess(pid)
	if err != nil {
		return false
	}
	if err := process.Signal(syscall.Signal(0)); err != nil {
		return false
	}
	return true
}

func seatunnelxJavaProxyPortFromEndpoint(endpoint string) int {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return 0
	}
	port, err := strconv.Atoi(parsed.Port())
	if err != nil || port <= 0 {
		return 0
	}
	return port
}

func seatunnelxJavaProxyPIDByPort(ctx context.Context, port int) int {
	if port <= 0 {
		return 0
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	commands := []string{
		fmt.Sprintf("lsof -ti tcp:%d -sTCP:LISTEN | head -n1", port),
		fmt.Sprintf(`ss -ltnp '( sport = :%d )' | sed -n '2p' | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p'`, port),
	}
	for _, shellCmd := range commands {
		cmd := exec.CommandContext(lookupCtx, "bash", "-lc", shellCmd)
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		pid, err := strconv.Atoi(strings.TrimSpace(string(output)))
		if err == nil && pid > 0 {
			return pid
		}
	}
	return 0
}

func defaultSeatunnelXJavaProxyVersion(version string) string {
	return firstNonBlank(strings.TrimSpace(version), seatunnelmeta.DefaultSeatunnelXJavaProxyVersion)
}

func seatunnelxJavaProxyErrorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
