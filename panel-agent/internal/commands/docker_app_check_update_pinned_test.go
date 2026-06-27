package commands

import (
	"context"
	"encoding/json"
	"testing"
)

// TestCheckUpdatePinnedUsesEmbeddedDigest covers GH #458 step 4: for a
// digest-pinned image_channel the check uses the embedded target digest as the
// "available" digest (no registry round-trip), so the comparison is against the
// reviewed catalog target rather than a mutable tag's current digest.
func TestCheckUpdatePinnedUsesEmbeddedDigest(t *testing.T) {
	const dig = "sha256:0123456789abcdef0123456789abcdef0123456789abcdef0123456789abcdef"
	params, _ := json.Marshal(map[string]any{
		"slug":          "vaultwarden",
		"image_channel": "vaultwarden/server:1.36.0@" + dig,
	})
	out, err := dockerAppCheckUpdateHandler(context.Background(), params)
	if err != nil {
		t.Fatalf("handler error: %v", err)
	}
	resp, ok := out.(dockerAppCheckUpdateResponse)
	if !ok {
		t.Fatalf("unexpected response type %T", out)
	}
	if resp.AvailableDigest != dig {
		t.Errorf("available_digest = %q, want the embedded %q", resp.AvailableDigest, dig)
	}
	// No running container in the test env → current empty → no false update.
	if resp.UpdateAvailable {
		t.Errorf("update_available should be false when current digest is unknown")
	}
}
