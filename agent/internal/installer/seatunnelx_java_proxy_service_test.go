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
	"net"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestSeatunnelXJavaProxyCmdlineMatchesInstallDir(t *testing.T) {
	cmdline := "java -Dseatunnelx.java.proxy.seatunnel.home=/opt/seatunnel-a -cp app org.apache.seatunnel.tools.proxy.SeatunnelXJavaProxyApplication"

	if !seatunnelxJavaProxyCmdlineMatchesInstallDir(cmdline, "/opt/seatunnel-a") {
		t.Fatal("expected java-proxy cmdline to match the same install_dir")
	}
	if !seatunnelxJavaProxyCmdlineMatchesInstallDir(cmdline, "/opt/seatunnel-a/") {
		t.Fatal("expected java-proxy cmdline to match normalized install_dir")
	}
	if seatunnelxJavaProxyCmdlineMatchesInstallDir(cmdline, "/opt/seatunnel-b") {
		t.Fatal("expected java-proxy cmdline not to match another install_dir")
	}
	if seatunnelxJavaProxyCmdlineMatchesInstallDir("java -jar unrelated.jar", "/opt/seatunnel-a") {
		t.Fatal("expected unrelated cmdline not to match java-proxy")
	}
}

func TestSeatunnelXJavaProxyPortCandidatesUseConfiguredDefaultFirst(t *testing.T) {
	stateDir := t.TempDir()
	ctx := WithSeatunnelXJavaProxyDefaultPort(context.Background(), 19080)

	candidates := seatunnelxJavaProxyPortCandidates(ctx, stateDir)
	if len(candidates) == 0 || candidates[0] != 19080 {
		t.Fatalf("expected configured default port to be first, got %#v", candidates)
	}
}

func TestFindOpenSeatunnelXJavaProxyPortIncrementsFromConflict(t *testing.T) {
	listener, err := net.Listen("tcp", net.JoinHostPort(seatunnelxJavaProxyDefaultHost, "0"))
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()

	addr, ok := listener.Addr().(*net.TCPAddr)
	if !ok {
		t.Fatalf("expected TCP address, got %T", listener.Addr())
	}
	port, err := findOpenSeatunnelXJavaProxyPort(addr.Port)
	if err != nil {
		t.Fatal(err)
	}
	if port <= addr.Port {
		t.Fatalf("expected open port to increment past occupied %s, got %d", strconv.Itoa(addr.Port), port)
	}
}

func TestForceStopManagedSeatunnelXJavaProxyServiceCleansStaleState(t *testing.T) {
	installDir := t.TempDir()
	stateDir := seatunnelxJavaProxyServiceStateDir(installDir)
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		t.Fatal(err)
	}
	portPath := filepath.Join(stateDir, "service.port")
	pidPath := filepath.Join(stateDir, "service.pid")
	if err := os.WriteFile(portPath, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pidPath, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	status, err := ForceStopManagedSeatunnelXJavaProxyService(context.Background(), installDir)
	if err != nil {
		t.Fatal(err)
	}
	if status.Running || status.Healthy {
		t.Fatalf("expected stale state to be treated as stopped, got %#v", status)
	}
	if _, err := os.Stat(portPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale port state to be removed, got err=%v", err)
	}
	if _, err := os.Stat(pidPath); !os.IsNotExist(err) {
		t.Fatalf("expected stale pid state to be removed, got err=%v", err)
	}
}
