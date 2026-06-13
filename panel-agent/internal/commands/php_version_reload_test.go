package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPHPVersionReload_ReloadsPerUserWorkersForVersion(t *testing.T) {
	// GH#180: reload targets the per-user jabali-fpm@<user> workers pinned
	// to the version, NOT the masked global php<v>-fpm.service.
	origRun, origPinned := runSystemctl, userPinnedToVersion
	defer func() { runSystemctl, userPinnedToVersion = origRun, origPinned }()

	var reloaded []string
	runSystemctl = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "list-units" {
			return []byte("jabali-fpm@alice.service loaded active running PHP-FPM\njabali-fpm@bob.service loaded active running PHP-FPM\n"), nil
		}
		if len(args) >= 2 && args[0] == "reload-or-restart" {
			reloaded = append(reloaded, args[1])
			return nil, nil
		}
		return nil, nil
	}
	// alice pinned to 8.5; bob is on a different version.
	userPinnedToVersion = func(user, version string) bool { return user == "alice" && version == "8.5" }

	params, _ := json.Marshal(phpVersionReloadParams{Version: "8.5"})
	_, err := phpVersionReloadHandler(context.Background(), params)
	require.NoError(t, err)
	require.Equal(t, []string{"jabali-fpm@alice.service"}, reloaded, "only the version-matched worker reloaded; masked global untouched")
}

func TestPHPVersionReload_NoWorkersIsSuccess(t *testing.T) {
	origRun, origPinned := runSystemctl, userPinnedToVersion
	defer func() { runSystemctl, userPinnedToVersion = origRun, origPinned }()
	runSystemctl = func(_ context.Context, _ ...string) ([]byte, error) { return nil, nil }
	userPinnedToVersion = func(_, _ string) bool { return false }

	params, _ := json.Marshal(phpVersionReloadParams{Version: "8.5"})
	_, err := phpVersionReloadHandler(context.Background(), params)
	require.NoError(t, err, "no active workers for the version is not an error")
}

func TestPHPVersionReload_SurfacesWorkerFailure(t *testing.T) {
	origRun, origPinned := runSystemctl, userPinnedToVersion
	defer func() { runSystemctl, userPinnedToVersion = origRun, origPinned }()
	runSystemctl = func(_ context.Context, args ...string) ([]byte, error) {
		if len(args) > 0 && args[0] == "list-units" {
			return []byte("jabali-fpm@alice.service loaded active running PHP-FPM\n"), nil
		}
		return []byte("Job failed"), fmt.Errorf("exit status 1")
	}
	userPinnedToVersion = func(_, _ string) bool { return true }

	params, _ := json.Marshal(phpVersionReloadParams{Version: "8.5"})
	_, err := phpVersionReloadHandler(context.Background(), params)
	require.Error(t, err, "a genuine per-user reload failure must surface")
}

func TestPHPVersionReload_InvalidVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		wantErr string
	}{
		{"unsupported", "5.6", "unsupported version"},
		{"invalid format", "8", "invalid version format"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			params, _ := json.Marshal(phpVersionReloadParams{Version: tt.version})

			_, err := phpVersionReloadHandler(ctx, params)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tt.wantErr)
		})
	}
}

func TestPHPVersionReload_NoParams(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	_, err := phpVersionReloadHandler(ctx, nil)
	require.Error(t, err)
	assert.Contains(t, err.Error(), "version parameter required")
}
