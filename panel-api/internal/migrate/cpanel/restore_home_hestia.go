package cpanel

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"git.jabali-panel.com/shukivaknin/jabali2/panel-api/internal/agent"
)

// HestiaHomeResult is returned to the restore-stage caller.
type HestiaHomeResult struct {
	BytesCopied   int64    `json:"bytes_copied"`
	Files         int64    `json:"files"`
	DomainsCopied int      `json:"domains_copied"`
	Skipped       []string `json:"skipped,omitempty"`
}

// ImportHomeHestia copies each Hestia `web/<domain>/public_html` into the Jabali
// docroot `/home/<user>/domains/<domain>/public_html` (GH 327) — the same path stored in
// domains.doc_root by the adapter (JAB-26).
//
// The generic legacy home rsync (ImportHome) copies the WebRoot's children into
// /home/<user>/ directly, so Hestia sites landed at /home/<user>/<domain>/…
// while the real docroot at /home/<user>/web/<domain>/public_html held only a
// generated placeholder index.html. This dispatches one agent rsync per domain
// with a `web/<dom>/public_html` dest_subpath, so content lands in the configured
// docroot and no duplicate /home/<user>/<domain> tree is created.
//
// parsed.HomeDir is the source WebRoot (<extractDir>/web); parsed.DomainNames is
// the hosted-domain list the adapter derived from the WebRoot scan.
func ImportHomeHestia(
	ctx context.Context,
	agentCaller agent.AgentInterface,
	parsed *ParsedTarball,
	jobID, destUser string,
) (*HestiaHomeResult, error) {
	if agentCaller == nil {
		return nil, fmt.Errorf("ImportHomeHestia: agent caller nil")
	}
	if parsed == nil {
		return nil, fmt.Errorf("ImportHomeHestia: parsed nil")
	}
	if jobID == "" || destUser == "" {
		return nil, fmt.Errorf("ImportHomeHestia: jobID + destUser required")
	}
	res := &HestiaHomeResult{}
	if parsed.HomeDir == "" || len(parsed.DomainNames) == 0 {
		res.Skipped = append(res.Skipped, "hestia_home_skip:no_web_domains")
		return res, nil
	}
	for _, dom := range parsed.DomainNames {
		src := filepath.Join(parsed.HomeDir, dom, "public_html")
		if st, err := os.Stat(src); err != nil || !st.IsDir() {
			res.Skipped = append(res.Skipped, fmt.Sprintf("hestia_home_skip:%s no public_html at %s", dom, src))
			continue
		}
		raw, err := agentCaller.Call(ctx, "migration.import_home", map[string]any{
			"job_id":       jobID,
			"src_dir":      src,
			"dest_user":    destUser,
			"dest_subpath": filepath.Join("domains", dom, "public_html"),
		})
		if err != nil {
			res.Skipped = append(res.Skipped, fmt.Sprintf("hestia_home_skip:%s rsync:%v", dom, err))
			continue
		}
		var r struct {
			BytesCopied int64 `json:"bytes_copied"`
			Files       int64 `json:"files"`
		}
		if jErr := json.Unmarshal(raw, &r); jErr == nil {
			res.BytesCopied += r.BytesCopied
			res.Files += r.Files
		}
		res.DomainsCopied++
	}
	return res, nil
}
