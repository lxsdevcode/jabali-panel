package commands

import (
	"context"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// captureSystemctl swaps in a runner that records every invocation and answers
// LoadState/is-active/is-enabled probes from `loaded`. socketLoad controls
// whether companion .socket units report as installed.
func captureSystemctl(t *testing.T, socketLoad string) *[][]string {
	t.Helper()
	orig := systemctlRunner
	t.Cleanup(func() { systemctlRunner = orig })
	calls := &[][]string{}
	systemctlRunner = func(_ context.Context, args ...string) (string, error) {
		*calls = append(*calls, args)
		switch args[0] {
		case "show": // show -p LoadState --value <unit>
			unit := args[len(args)-1]
			if strings.HasSuffix(unit, ".socket") {
				return socketLoad, nil
			}
			return "loaded", nil
		case "is-enabled":
			return "enabled", nil
		case "is-active":
			return "inactive", nil
		}
		return "", nil
	}
	return calls
}

func verbCall(calls [][]string, verb string) []string {
	for _, c := range calls {
		if len(c) > 0 && c[0] == verb {
			return c
		}
	}
	return nil
}

// GH #370: stopping a socket-activated service must also stop its .socket, or
// the next connection re-triggers the daemon.
func TestRunServiceAction_StopsDockerSocketCompanion(t *testing.T) {
	calls := captureSystemctl(t, "loaded") // docker.socket installed
	_, err := runServiceAction(context.Background(), "stop", "docker")
	require.NoError(t, err)
	assert.Equal(t, []string{"stop", "docker.service", "docker.socket"}, verbCall(*calls, "stop"))
}

// enable/disable move the companion too, so socket-activation persistence
// tracks the service.
func TestRunServiceAction_DisableIncludesSocket(t *testing.T) {
	calls := captureSystemctl(t, "loaded")
	_, err := runServiceAction(context.Background(), "disable", "docker")
	require.NoError(t, err)
	assert.Equal(t, []string{"disable", "docker.service", "docker.socket"}, verbCall(*calls, "disable"))
}

// A companion that isn't installed on the host is skipped (no systemctl error
// on a not-found unit).
func TestRunServiceAction_SkipsAbsentCompanion(t *testing.T) {
	calls := captureSystemctl(t, "not-found") // docker.socket absent
	_, err := runServiceAction(context.Background(), "stop", "docker")
	require.NoError(t, err)
	assert.Equal(t, []string{"stop", "docker.service"}, verbCall(*calls, "stop"))
}

// A service with no companions acts on its .service unit only.
func TestRunServiceAction_NoCompanionForPlainService(t *testing.T) {
	calls := captureSystemctl(t, "loaded")
	_, err := runServiceAction(context.Background(), "stop", "nginx")
	require.NoError(t, err)
	assert.Equal(t, []string{"stop", "nginx.service"}, verbCall(*calls, "stop"))
}

// Safety invariant: pdns-recursor must never be a companion — it owns
// 127.0.0.1:53 / the host resolver, so a UI toggle must not stop it.
func TestCompanionUnits_ExcludeRecursor(t *testing.T) {
	if _, ok := companionUnits["pdns"]; ok {
		t.Error("pdns must not have companion units — pdns-recursor is the host resolver")
	}
	for svc, comps := range companionUnits {
		for _, c := range comps {
			if strings.Contains(c, "recursor") {
				t.Errorf("service %q lists recursor companion %q — stopping it breaks host DNS", svc, c)
			}
		}
	}
}
