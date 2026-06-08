package commands

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

// docker_app.read_env returns the KEY=VALUE pairs from an install's on-disk
// .env so panel-api can re-render the compose template (on update / edit)
// WITHOUT regenerating the install's secrets. The .env is the source of truth
// for generated secrets (MASTERKEY, DB passwords, JWT secrets): panel-api
// materialised them at install time but never persisted them, so a fresh
// MaterialiseEnv would mint new values and break any stateful service. Reading
// them back over the root-only UDS keeps the existing secrets stable.
//
// Response carries secret values — callers MUST NOT log it.

type dockerAppReadEnvParams struct {
	Slug string `json:"slug"`
}

type dockerAppReadEnvResponse struct {
	Slug string            `json:"slug"`
	Env  map[string]string `json:"env"`
}

func dockerAppReadEnvHandler(_ context.Context, params json.RawMessage) (any, error) {
	var p dockerAppReadEnvParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse: %v", err)}
	}
	if err := validateSlug(p.Slug); err != nil {
		return nil, err
	}
	path := filepath.Join(dockerAppDataRoot, p.Slug, ".env")
	env, err := parseEnvFile(path)
	if err != nil {
		// No .env (some installs have no secrets) → return empty, not an
		// error: the caller treats "no env" as "nothing to preserve".
		if os.IsNotExist(err) {
			return dockerAppReadEnvResponse{Slug: p.Slug, Env: map[string]string{}}, nil
		}
		return nil, &agentwire.AgentError{Code: agentwire.CodeInternal, Message: fmt.Sprintf("read .env: %v", err)}
	}
	return dockerAppReadEnvResponse{Slug: p.Slug, Env: env}, nil
}

// parseEnvFile reads a KEY=VALUE .env (the format buildEnvFile writes:
// one pair per line, no quoting, value is everything after the first '=').
func parseEnvFile(path string) (map[string]string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	out := map[string]string{}
	sc := bufio.NewScanner(f)
	// Allow long secret lines.
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		i := strings.IndexByte(line, '=')
		if i <= 0 {
			continue
		}
		out[line[:i]] = line[i+1:]
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return out, nil
}
