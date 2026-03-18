package main

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	pb "github.com/seatunnel/seatunnelX/agent"
	"github.com/seatunnel/seatunnelX/agent/internal/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type noopProgressReporter struct{}

func (noopProgressReporter) Report(progress int32, output string) error {
	return nil
}

func TestHandleRemoveInstallDirCommandRejectsOutsideBaseDir(t *testing.T) {
	allowedBase := filepath.Join(t.TempDir(), "seatunnel")
	require.NoError(t, os.MkdirAll(allowedBase, 0o755))

	outsideDir := filepath.Join(t.TempDir(), "outside")
	require.NoError(t, os.MkdirAll(outsideDir, 0o755))

	agent := NewAgent(&config.Config{
		SeaTunnel: config.SeaTunnelConfig{InstallDir: allowedBase},
	})

	cmd := &pb.CommandRequest{
		CommandId:  "cmd-1",
		Parameters: map[string]string{"install_dir": outsideDir},
	}

	resp, err := agent.handleRemoveInstallDirCommand(context.Background(), cmd, noopProgressReporter{})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "outside allowed base dir")
	assert.Equal(t, pb.CommandStatus_FAILED, resp.Status)

	_, statErr := os.Stat(outsideDir)
	assert.NoError(t, statErr)
}

func TestHandleRemoveInstallDirCommandAllowsSubdirWithinBaseDir(t *testing.T) {
	allowedBase := filepath.Join(t.TempDir(), "seatunnel")
	targetDir := filepath.Join(allowedBase, "cluster-a")
	require.NoError(t, os.MkdirAll(targetDir, 0o755))

	agent := NewAgent(&config.Config{
		SeaTunnel: config.SeaTunnelConfig{InstallDir: allowedBase},
	})

	cmd := &pb.CommandRequest{
		CommandId:  "cmd-2",
		Parameters: map[string]string{"install_dir": targetDir},
	}

	resp, err := agent.handleRemoveInstallDirCommand(context.Background(), cmd, noopProgressReporter{})
	require.NoError(t, err)
	assert.Equal(t, pb.CommandStatus_SUCCESS, resp.Status)

	_, statErr := os.Stat(targetDir)
	assert.True(t, os.IsNotExist(statErr))
}

func TestIsAllowedInstallDirUsesDefaultBase(t *testing.T) {
	agent := NewAgent(&config.Config{})
	assert.True(t, agent.isAllowedInstallDir("/opt/seatunnel"))
	assert.True(t, agent.isAllowedInstallDir("/opt/seatunnel/cluster"))
	assert.False(t, agent.isAllowedInstallDir("/var/tmp/seatunnel"))
}
