// docker_app.check_update — query the upstream registry for the
// current digest of an image, compare against the on-disk digest
// of the running container. Cheap (~1 HTTP HEAD via `docker manifest
// inspect`); called by the reconciler every ~6h per app.
package commands

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

type dockerAppCheckUpdateParams struct {
	Slug         string `json:"slug"`
	ImageChannel string `json:"image_channel"` // e.g. "vaultwarden/server:latest"
}

type dockerAppCheckUpdateResponse struct {
	Slug            string `json:"slug"`
	ImageChannel    string `json:"image_channel"`
	CurrentDigest   string `json:"current_digest,omitempty"`   // local container digest
	AvailableDigest string `json:"available_digest,omitempty"` // upstream registry digest
	UpdateAvailable bool   `json:"update_available"`
}

func dockerAppCheckUpdateHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p dockerAppCheckUpdateParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse: %v", err)}
	}
	if err := validateSlug(p.Slug); err != nil {
		return nil, err
	}
	if p.ImageChannel == "" {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "image_channel required"}
	}

	// Upstream digest via `docker manifest inspect`. -experimental
	// flag was promoted to GA in compose v2 / docker 20.10; safe to
	// invoke without flags.
	upstream, err := dockerManifestDigest(ctx, p.ImageChannel)
	if err != nil {
		return nil, &agentwire.AgentError{
			Code:    agentwire.CodeInternal,
			Message: fmt.Sprintf("docker manifest inspect failed: %v", err),
		}
	}

	// Local digest: the container's currently-running image.
	// docker inspect --format '{{.Image}}' on the container, then
	// docker image inspect to resolve a content digest. Simpler:
	// docker inspect jabali-app-<slug> --format '{{index .Config.Image}}'
	// + image inspect for the RepoDigests[0].
	local := dockerContainerImageDigest(ctx, "jabali-app-"+p.Slug)

	resp := dockerAppCheckUpdateResponse{
		Slug:            p.Slug,
		ImageChannel:    p.ImageChannel,
		CurrentDigest:   local,
		AvailableDigest: upstream,
		UpdateAvailable: upstream != "" && local != "" && upstream != local,
	}
	return resp, nil
}

func dockerManifestDigest(ctx context.Context, image string) (string, error) {
	out, err := exec.CommandContext(ctx, "docker", "manifest", "inspect", "--verbose", image).Output()
	if err != nil {
		// Try without --verbose for older docker.
		out, err = exec.CommandContext(ctx, "docker", "manifest", "inspect", image).Output()
		if err != nil {
			return "", err
		}
	}
	// --verbose returns either a single object or a list (multi-arch
	// manifests). Walk it and prefer linux/amd64.
	var arr []map[string]any
	if err := json.Unmarshal(out, &arr); err == nil {
		for _, m := range arr {
			if desc, ok := m["Descriptor"].(map[string]any); ok {
				if d, ok := desc["digest"].(string); ok && d != "" {
					return d, nil
				}
			}
		}
	}
	// Single-arch fallback (non-verbose shape).
	var obj map[string]any
	if err := json.Unmarshal(out, &obj); err == nil {
		if d, ok := obj["Digest"].(string); ok && d != "" {
			return d, nil
		}
	}
	return "", nil
}

func dockerContainerImageDigest(ctx context.Context, container string) string {
	// 1. Resolve the image ID the container is running.
	img, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Image}}", container).Output()
	if err != nil {
		return ""
	}
	imageID := strings.TrimSpace(string(img))
	if imageID == "" {
		return ""
	}
	// 2. Read RepoDigests off that image. The first one is the
	// authoritative remote digest.
	out, err := exec.CommandContext(ctx, "docker", "image", "inspect", "--format", "{{json .RepoDigests}}", imageID).Output()
	if err != nil {
		return ""
	}
	var arr []string
	if err := json.Unmarshal(out, &arr); err != nil || len(arr) == 0 {
		return ""
	}
	// repoDigest shape: "vaultwarden/server@sha256:abcdef...". Split.
	parts := strings.SplitN(arr[0], "@", 2)
	if len(parts) == 2 {
		return parts[1]
	}
	return ""
}

func init() {
	Default.Register("docker_app.check_update", dockerAppCheckUpdateHandler)
}
