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
	"io"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/seatunnel/seatunnelX/agent/internal/logger"
	seatunnelmeta "github.com/seatunnel/seatunnelX/internal/seatunnel"
)

const seatunnelxJavaProxyStopTimeout = 8 * time.Second

// InstallOrRepairSeatunnelXJavaProxySupportAssets downloads the managed java-proxy support assets.
// InstallOrRepairSeatunnelXJavaProxySupportAssets 下载并修复托管 java-proxy 辅助资产。
func InstallOrRepairSeatunnelXJavaProxySupportAssets(
	ctx context.Context,
	supportDir string,
	version string,
	jarURL string,
	scriptURL string,
) (map[string]string, error) {
	resolvedSupportDir := strings.TrimSpace(supportDir)
	if resolvedSupportDir == "" {
		resolvedSupportDir = strings.TrimSpace(os.Getenv(seatunnelxJavaProxyHomeEnvVar))
	}
	if resolvedSupportDir == "" {
		resolvedSupportDir = seatunnelxJavaProxyDefaultSupportDir
	}
	resolvedVersion := seatunnelmeta.ResolveSeatunnelXJavaProxyVersion(version)
	if strings.TrimSpace(jarURL) == "" {
		return nil, fmt.Errorf("seatunnelx-java-proxy jar_url is required")
	}
	if strings.TrimSpace(scriptURL) == "" {
		return nil, fmt.Errorf("seatunnelx-java-proxy script_url is required")
	}

	libDir := filepath.Join(resolvedSupportDir, "lib")
	scriptDir := filepath.Join(resolvedSupportDir, "scripts")
	if err := os.MkdirAll(libDir, 0o755); err != nil {
		return nil, fmt.Errorf("create java-proxy lib dir: %w", err)
	}
	if err := os.MkdirAll(scriptDir, 0o755); err != nil {
		return nil, fmt.Errorf("create java-proxy script dir: %w", err)
	}

	jarPath := filepath.Join(libDir, seatunnelmeta.SeatunnelXJavaProxyJarFileName(resolvedVersion))
	scriptPath := filepath.Join(scriptDir, seatunnelmeta.SeatunnelXJavaProxyScriptFileName)
	if err := downloadSeatunnelXJavaProxyAsset(ctx, jarURL, jarPath, 0o644); err != nil {
		return nil, fmt.Errorf("install java-proxy jar: %w", err)
	}
	if err := downloadSeatunnelXJavaProxyAsset(ctx, scriptURL, scriptPath, 0o755); err != nil {
		return nil, fmt.Errorf("install java-proxy script: %w", err)
	}

	return map[string]string{
		"support_dir": resolvedSupportDir,
		"version":     resolvedVersion,
		"jar_path":    jarPath,
		"script_path": scriptPath,
	}, nil
}

func downloadSeatunnelXJavaProxyAsset(ctx context.Context, rawURL string, targetPath string, mode os.FileMode) error {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return fmt.Errorf("parse asset url: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("unsupported asset url scheme: %s", parsed.Scheme)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 2 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return fmt.Errorf("download returned status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	tmpFile, err := os.CreateTemp(filepath.Dir(targetPath), ".seatunnelx-java-proxy-*")
	if err != nil {
		return err
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)
	written, copyErr := io.Copy(tmpFile, resp.Body)
	closeErr := tmpFile.Close()
	if copyErr != nil {
		return copyErr
	}
	if closeErr != nil {
		return closeErr
	}
	if written <= 0 {
		return fmt.Errorf("downloaded asset is empty")
	}
	if err := os.Chmod(tmpPath, mode); err != nil {
		return err
	}
	if err := os.Rename(tmpPath, targetPath); err != nil {
		return err
	}
	return nil
}

// SeatunnelXJavaProxyServiceStatus describes the current managed seatunnelx-java-proxy state.
type SeatunnelXJavaProxyServiceStatus struct {
	Service    string `json:"service"`
	Managed    bool   `json:"managed"`
	Installed  bool   `json:"installed"`
	Running    bool   `json:"running"`
	Healthy    bool   `json:"healthy"`
	Endpoint   string `json:"endpoint,omitempty"`
	Port       int    `json:"port,omitempty"`
	PID        int    `json:"pid,omitempty"`
	InstallDir string `json:"install_dir,omitempty"`
	LogPath    string `json:"log_path,omitempty"`
	StateDir   string `json:"state_dir,omitempty"`
	Message    string `json:"message,omitempty"`
}

// StartManagedSeatunnelXJavaProxyService ensures the managed seatunnelx-java-proxy service is available.
func StartManagedSeatunnelXJavaProxyService(ctx context.Context, installDir string, seatunnelVersion string) (*SeatunnelXJavaProxyServiceStatus, error) {
	status, _ := GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir, seatunnelVersion)
	if status != nil && status.Healthy {
		return status, nil
	}

	baseURL, err := ensureSeatunnelXJavaProxyService(ctx, installDir, seatunnelVersion)
	if err != nil {
		return nil, err
	}

	status, statusErr := GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir, seatunnelVersion)
	if statusErr != nil {
		return &SeatunnelXJavaProxyServiceStatus{
			Service:    "seatunnelx_java_proxy",
			Managed:    true,
			Installed:  true,
			Running:    true,
			Healthy:    true,
			Endpoint:   baseURL,
			InstallDir: installDir,
			Message:    "seatunnelx-java-proxy service started",
			StateDir:   seatunnelxJavaProxyServiceStateDir(installDir),
			LogPath:    filepath.Join(seatunnelxJavaProxyServiceStateDir(installDir), "service.log"),
		}, nil
	}
	status.Message = firstNonBlank(status.Message, "seatunnelx-java-proxy service started")
	return status, nil
}

// GetManagedSeatunnelXJavaProxyServiceStatus returns the current seatunnelx-java-proxy state.
func GetManagedSeatunnelXJavaProxyServiceStatus(ctx context.Context, installDir string, seatunnelVersion ...string) (*SeatunnelXJavaProxyServiceStatus, error) {
	version := seatunnelmeta.ResolveSeatunnelXJavaProxyVersion(firstNonBlank(seatunnelVersion...))
	status := &SeatunnelXJavaProxyServiceStatus{
		Service:    "seatunnelx_java_proxy",
		Managed:    true,
		Installed:  seatunnelxJavaProxySupportAssetsInstalled(installDir, version),
		InstallDir: installDir,
		StateDir:   seatunnelxJavaProxyServiceStateDir(installDir),
		LogPath:    filepath.Join(seatunnelxJavaProxyServiceStateDir(installDir), "service.log"),
	}

	if endpoint := strings.TrimSpace(os.Getenv(seatunnelxJavaProxyEndpointEnvVar)); endpoint != "" {
		normalized := strings.TrimRight(endpoint, "/")
		status.Managed = false
		status.Installed = true
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
			if seatunnelxJavaProxyPIDMatchesInstallDir(pid, installDir) {
				status.PID = pid
			}
		}
	}

	if status.Port > 0 && !seatunnelxJavaProxyPortOwnedByInstallDir(ctx, status.Port, installDir) {
		status.Endpoint = ""
		status.Port = 0
		status.PID = 0
		status.Message = "seatunnelx-java-proxy port is owned by another install_dir"
	}

	if status.Endpoint == "" {
		for _, port := range seatunnelxJavaProxyPortCandidates(ctx, status.StateDir) {
			if port <= 0 {
				continue
			}
			endpoint := seatunnelxJavaProxyServiceBaseURL(port)
			if err := waitForSeatunnelXJavaProxyHealthy(ctx, endpoint, 1200*time.Millisecond); err == nil {
				if !seatunnelxJavaProxyPortOwnedByInstallDir(ctx, port, installDir) {
					continue
				}
				status.Endpoint = endpoint
				status.Port = port
				status.Healthy = true
				status.Running = true
				if pid := seatunnelxJavaProxyPIDByPortForInstallDir(ctx, port, installDir); pid > 0 {
					status.PID = pid
				}
				_ = os.MkdirAll(status.StateDir, 0o755)
				_ = os.WriteFile(filepath.Join(status.StateDir, "service.port"), []byte(strconv.Itoa(port)+"\n"), 0o644)
				break
			}
		}
	}

	if status.PID <= 0 && status.Port > 0 {
		if pid := seatunnelxJavaProxyPIDByPortForInstallDir(ctx, status.Port, installDir); pid > 0 {
			status.PID = pid
			_ = os.MkdirAll(status.StateDir, 0o755)
			_ = os.WriteFile(filepath.Join(status.StateDir, "service.pid"), []byte(strconv.Itoa(pid)+"\n"), 0o644)
		}
	}
	if status.PID > 0 {
		status.Running = seatunnelxJavaProxyPIDAlive(status.PID)
	}
	if status.Endpoint != "" && !status.Healthy {
		if err := waitForSeatunnelXJavaProxyHealthy(ctx, status.Endpoint, 1500*time.Millisecond); err == nil &&
			seatunnelxJavaProxyPortOwnedByInstallDir(ctx, status.Port, installDir) {
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
		status.Message = firstNonBlank(status.Message, "seatunnelx-java-proxy service is not running")
	}

	return status, nil
}

func seatunnelxJavaProxySupportAssetsInstalled(installDir string, seatunnelVersion string) bool {
	if _, err := resolveSeatunnelXJavaProxyScriptPath(installDir); err != nil {
		return false
	}
	if _, err := resolveSeatunnelXJavaProxyJarPath(installDir, seatunnelVersion); err != nil {
		return false
	}
	return true
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
		logger.WarnF(ctx, "[seatunnelx-java-proxy] graceful stop timed out, force killing managed process: pid=%d, port=%d, error=%v", status.PID, status.Port, waitErr)
		if killErr := forceKillSeatunnelXJavaProxyProcesses(ctx, status); killErr != nil {
			status.Message = "failed to stop seatunnelx-java-proxy service"
			return status, fmt.Errorf("%w; force kill failed: %v", waitErr, killErr)
		}
	}

	_ = os.Remove(filepath.Join(status.StateDir, "service.pid"))
	stoppedStatus, statusErr := GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir)
	if statusErr != nil {
		return &SeatunnelXJavaProxyServiceStatus{
			Service:    "seatunnelx_java_proxy",
			Managed:    true,
			Installed:  status.Installed,
			Running:    false,
			Healthy:    false,
			Endpoint:   status.Endpoint,
			Port:       status.Port,
			InstallDir: installDir,
			LogPath:    status.LogPath,
			StateDir:   status.StateDir,
			Message:    "seatunnelx-java-proxy service stopped",
		}, nil
	}
	stoppedStatus.Message = "seatunnelx-java-proxy service stopped"
	return stoppedStatus, nil
}

// ForceStopManagedSeatunnelXJavaProxyService 强制终止本地托管的 seatunnelx-java-proxy 服务。
// ForceStopManagedSeatunnelXJavaProxyService forcefully kills the locally managed seatunnelx-java-proxy service.
func ForceStopManagedSeatunnelXJavaProxyService(ctx context.Context, installDir string) (*SeatunnelXJavaProxyServiceStatus, error) {
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
		cleanupSeatunnelXJavaProxyRuntimeState(status)
		status.Message = "seatunnelx-java-proxy service is already stopped"
		return status, nil
	}
	if err := forceKillSeatunnelXJavaProxyProcesses(ctx, status); err != nil {
		status.Message = "failed to force stop seatunnelx-java-proxy service"
		return status, err
	}

	cleanupSeatunnelXJavaProxyRuntimeState(status)
	stoppedStatus, statusErr := GetManagedSeatunnelXJavaProxyServiceStatus(ctx, installDir)
	if statusErr != nil {
		return &SeatunnelXJavaProxyServiceStatus{
			Service:    "seatunnelx_java_proxy",
			Managed:    true,
			Installed:  status.Installed,
			Running:    false,
			Healthy:    false,
			Endpoint:   status.Endpoint,
			Port:       status.Port,
			InstallDir: installDir,
			LogPath:    status.LogPath,
			StateDir:   status.StateDir,
			Message:    "seatunnelx-java-proxy service force killed",
		}, nil
	}
	stoppedStatus.Message = "seatunnelx-java-proxy service force killed"
	return stoppedStatus, nil
}

// cleanupSeatunnelXJavaProxyRuntimeState 清理运行时 PID/端口状态，避免卸载后残留。
// cleanupSeatunnelXJavaProxyRuntimeState removes runtime PID/port state to avoid stale uninstall leftovers.
func cleanupSeatunnelXJavaProxyRuntimeState(status *SeatunnelXJavaProxyServiceStatus) {
	if status == nil || strings.TrimSpace(status.StateDir) == "" {
		return
	}
	_ = os.Remove(filepath.Join(status.StateDir, "service.pid"))
	_ = os.Remove(filepath.Join(status.StateDir, "service.port"))
}

// forceKillSeatunnelXJavaProxyProcesses 在优雅停止超时后强制终止已知的 proxy 进程。
// forceKillSeatunnelXJavaProxyProcesses forcefully terminates known proxy processes after graceful shutdown times out.
func forceKillSeatunnelXJavaProxyProcesses(ctx context.Context, status *SeatunnelXJavaProxyServiceStatus) error {
	if status == nil {
		return fmt.Errorf("seatunnelx-java-proxy service status is unavailable")
	}

	pids := make([]int, 0, 4)
	if status.PID > 0 {
		pids = append(pids, status.PID)
	}
	if status.Port > 0 {
		pids = append(pids, seatunnelxJavaProxyPIDsByPort(ctx, status.Port)...)
	}
	pids = uniquePositivePIDs(pids)
	if len(pids) == 0 {
		return fmt.Errorf("no seatunnelx-java-proxy process found to force kill")
	}

	killed := 0
	for _, pid := range pids {
		if !seatunnelxJavaProxyPIDAlive(pid) {
			continue
		}
		// 端口发现的 PID 需要再次校验命令行和 install_dir，避免误杀其他集群实例。
		// PIDs discovered from the port are validated by cmdline and install_dir to avoid killing another cluster instance.
		if !seatunnelxJavaProxyPIDMatchesInstallDir(pid, status.InstallDir) {
			logger.WarnF(ctx, "[seatunnelx-java-proxy] skip force killing non-matching listener: pid=%d, port=%d, install_dir=%s", pid, status.Port, status.InstallDir)
			continue
		}
		if err := syscall.Kill(pid, syscall.SIGKILL); err != nil && !errors.Is(err, os.ErrProcessDone) && !errors.Is(err, syscall.ESRCH) {
			return fmt.Errorf("force kill seatunnelx-java-proxy pid %d: %w", pid, err)
		}
		killed++
		logger.WarnF(ctx, "[seatunnelx-java-proxy] sent SIGKILL to managed process: pid=%d", pid)
	}
	if killed == 0 {
		return fmt.Errorf("no live seatunnelx-java-proxy process was force killed")
	}

	if waitErr := waitForSeatunnelXJavaProxyShutdown(ctx, status.Endpoint, status.PID, 3*time.Second); waitErr != nil {
		return waitErr
	}
	return nil
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
	if runtime.GOOS == "linux" {
		if state, ok := linuxProcessState(pid); ok && (state == "Z" || state == "X") {
			return false
		}
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

// linuxProcessState 读取 /proc/<pid>/stat 并返回 Linux 单字符进程状态。
// linuxProcessState reads /proc/<pid>/stat and returns the one-letter Linux process state.
func linuxProcessState(pid int) (string, bool) {
	statPath := filepath.Join("/proc", strconv.Itoa(pid), "stat")
	content, err := os.ReadFile(statPath)
	if err != nil {
		return "", false
	}
	fields := strings.Fields(string(content))
	if len(fields) < 3 {
		return "", false
	}
	return fields[2], true
}

// seatunnelxJavaProxyPIDMatches 检查进程命令行是否属于 seatunnelx-java-proxy。
// seatunnelxJavaProxyPIDMatches checks whether a process command line belongs to seatunnelx-java-proxy.
func seatunnelxJavaProxyPIDMatches(pid int) bool {
	cmdline, ok := seatunnelxJavaProxyPIDCmdline(pid)
	if !ok {
		return false
	}
	return strings.Contains(cmdline, "SeatunnelXJavaProxyApplication") ||
		strings.Contains(cmdline, "seatunnelx-java-proxy")
}

func seatunnelxJavaProxyPIDMatchesInstallDir(pid int, installDir string) bool {
	cmdline, ok := seatunnelxJavaProxyPIDCmdline(pid)
	if !ok {
		return false
	}
	return seatunnelxJavaProxyCmdlineMatchesInstallDir(cmdline, installDir)
}

func seatunnelxJavaProxyPIDCmdline(pid int) (string, bool) {
	cmdlinePath := filepath.Join("/proc", strconv.Itoa(pid), "cmdline")
	content, err := os.ReadFile(cmdlinePath)
	if err != nil {
		return "", false
	}
	return strings.ReplaceAll(string(content), "\x00", " "), true
}

func seatunnelxJavaProxyCmdlineMatchesInstallDir(cmdline string, installDir string) bool {
	if strings.TrimSpace(cmdline) == "" || !seatunnelxJavaProxyCmdlineMatchesProxy(cmdline) {
		return false
	}
	trimmedInstallDir := strings.TrimSpace(installDir)
	normalizedInstallDir := filepath.Clean(trimmedInstallDir)
	if normalizedInstallDir == "" || normalizedInstallDir == "." {
		return true
	}
	if strings.Contains(cmdline, "-Dseatunnelx.java.proxy.seatunnel.home="+normalizedInstallDir) {
		return true
	}
	return trimmedInstallDir != normalizedInstallDir &&
		strings.Contains(cmdline, "-Dseatunnelx.java.proxy.seatunnel.home="+trimmedInstallDir)
}

func seatunnelxJavaProxyCmdlineMatchesProxy(cmdline string) bool {
	return strings.Contains(cmdline, "SeatunnelXJavaProxyApplication") ||
		strings.Contains(cmdline, "seatunnelx-java-proxy")
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
	pids := seatunnelxJavaProxyPIDsByPort(ctx, port)
	if len(pids) == 0 {
		return 0
	}
	return pids[0]
}

func seatunnelxJavaProxyPIDByPortForInstallDir(ctx context.Context, port int, installDir string) int {
	for _, pid := range seatunnelxJavaProxyPIDsByPort(ctx, port) {
		if seatunnelxJavaProxyPIDMatchesInstallDir(pid, installDir) {
			return pid
		}
	}
	return 0
}

func seatunnelxJavaProxyPortOwnedByInstallDir(ctx context.Context, port int, installDir string) bool {
	pids := seatunnelxJavaProxyPIDsByPort(ctx, port)
	if len(pids) == 0 {
		return true
	}
	for _, pid := range pids {
		if seatunnelxJavaProxyPIDMatchesInstallDir(pid, installDir) {
			return true
		}
	}
	return false
}

// seatunnelxJavaProxyPIDsByPort 返回指定 proxy 端口上的监听进程 PID。
// seatunnelxJavaProxyPIDsByPort returns listener PIDs on the given proxy port.
func seatunnelxJavaProxyPIDsByPort(ctx context.Context, port int) []int {
	if port <= 0 {
		return nil
	}
	lookupCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	commands := []string{
		fmt.Sprintf("lsof -ti tcp:%d -sTCP:LISTEN", port),
		fmt.Sprintf(`ss -ltnp '( sport = :%d )' | sed -n '2,$p' | sed -n 's/.*pid=\([0-9][0-9]*\).*/\1/p'`, port),
	}
	var pids []int
	for _, shellCmd := range commands {
		cmd := exec.CommandContext(lookupCtx, "bash", "-lc", shellCmd)
		output, err := cmd.Output()
		if err != nil {
			continue
		}
		for _, field := range strings.Fields(strings.TrimSpace(string(output))) {
			pid, err := strconv.Atoi(field)
			if err == nil && pid > 0 {
				pids = append(pids, pid)
			}
		}
	}
	return uniquePositivePIDs(pids)
}

// uniquePositivePIDs 按发现顺序去重正数进程 ID。
// uniquePositivePIDs deduplicates process IDs while preserving discovery order.
func uniquePositivePIDs(pids []int) []int {
	seen := make(map[int]struct{}, len(pids))
	result := make([]int, 0, len(pids))
	for _, pid := range pids {
		if pid <= 0 {
			continue
		}
		if _, ok := seen[pid]; ok {
			continue
		}
		seen[pid] = struct{}{}
		result = append(result, pid)
	}
	return result
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
