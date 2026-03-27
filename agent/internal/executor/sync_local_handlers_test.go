package executor

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFilterLogContentSupportsKeywordAndLevel(t *testing.T) {
	content := strings.Join([]string{
		"2026-03-27 10:00:00,000 INFO startup complete",
		"2026-03-27 10:00:01,000 WARN slow source",
		"2026-03-27 10:00:02,000 ERROR task failed",
	}, "\n")

	got := filterLogContent(content, "source", "warn")
	if !strings.Contains(got, "WARN slow source") {
		t.Fatalf("expected warn line to remain, got %q", got)
	}
	if strings.Contains(got, "ERROR") || strings.Contains(got, "startup") {
		t.Fatalf("expected unrelated lines filtered out, got %q", got)
	}
}

func TestReadSyncLogChunkReturnsIncrementalContent(t *testing.T) {
	logFile := filepath.Join(t.TempDir(), "job.log")
	content := "line1\nline2\nline3\n"
	if err := os.WriteFile(logFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}
	logs, nextOffset, fileSize, err := readSyncLogChunk(logFile, "0", 12, "", "")
	if err != nil {
		t.Fatalf("readSyncLogChunk returned error: %v", err)
	}
	if logs != "line1\nline2" {
		t.Fatalf("unexpected chunk logs %q", logs)
	}
	if nextOffset <= 0 || fileSize != int64(len(content)) {
		t.Fatalf("unexpected offsets next=%d size=%d", nextOffset, fileSize)
	}
}

func TestResolveClusterJobLogFilePrefersLogsDirectory(t *testing.T) {
	baseDir := t.TempDir()
	logDir := filepath.Join(baseDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		t.Fatalf("mkdir logs: %v", err)
	}
	target := filepath.Join(logDir, "job-177.log")
	if err := os.WriteFile(target, []byte("demo"), 0o644); err != nil {
		t.Fatalf("write log file: %v", err)
	}
	got, err := resolveClusterJobLogFile(baseDir, []string{"177"})
	if err != nil {
		t.Fatalf("resolveClusterJobLogFile returned error: %v", err)
	}
	if got != target {
		t.Fatalf("expected %s, got %s", target, got)
	}
}
