// backup_dns_read.go — GH #267 DNS restore (read half).
//
// Reads the captured user DNS records out of a backup's metadata.json (the
// stage=meta snapshot) for a set of domains, so the panel can re-insert them.
// Read-only; the panel does the owner-scoped DB writes + reconcile.
package commands

import (
	"context"
	"encoding/json"
	"fmt"

	"git.jabali-panel.com/shukivaknin/jabali2/internal/backup"
)

type backupDNSReadParams struct {
	ManifestSnapshotID string   `json:"manifest_snapshot_id"`
	DomainNames        []string `json:"domain_names"`

	RepoURL        string            `json:"repo_url,omitempty"`
	CredentialsRef string            `json:"credentials_ref,omitempty"`
	PasswordFile   string            `json:"password_file,omitempty"`
	SFTP           *backupSFTPInputs `json:"sftp,omitempty"`
}

type backupDNSDomain struct {
	Name       string                     `json:"name"`
	DNSRecords []backup.MetadataDNSRecord `json:"dns_records"`
}

type backupDNSReadResult struct {
	Domains []backupDNSDomain `json:"domains"`
}

func backupDNSReadHandler(ctx context.Context, raw json.RawMessage) (any, error) {
	var p backupDNSReadParams
	if err := json.Unmarshal(raw, &p); err != nil {
		return nil, bkInvalidArg("malformed JSON body")
	}
	if p.ManifestSnapshotID == "" {
		return nil, bkInvalidArg("manifest_snapshot_id required")
	}
	if len(p.DomainNames) == 0 {
		return nil, bkInvalidArg("domain_names required")
	}

	// Same destination wire contract as backup.create — empty falls back
	// to the legacy default repo (GH #1044: hardcoding the default made
	// DNS restore read the wrong repo for per-destination backups).
	cfg, cerr := bkResticConfigWithPassword(p.RepoURL, p.CredentialsRef, p.PasswordFile, p.SFTP)
	if cerr != nil {
		return nil, bkInternal("restic config", cerr)
	}
	c := backup.New(cfg)

	manifestBytes, err := c.Dump(ctx, p.ManifestSnapshotID, "manifest.json")
	if err != nil {
		return nil, bkInternal("read manifest", err)
	}
	manifest, err := backup.AccountManifestFromBytes(manifestBytes)
	if err != nil {
		return nil, bkInternal("parse manifest", err)
	}

	// Locate the meta stage snapshot, which holds metadata.json (the panel-side
	// rows captured at backup time, including each domain's user DNS records).
	metaSnap := ""
	for _, st := range manifest.Stages {
		if st.Name == backup.StageMeta && st.Status == backup.StageStatusOK && st.SnapshotID != "" {
			metaSnap = st.SnapshotID
			break
		}
	}
	if metaSnap == "" {
		return backupDNSReadResult{Domains: []backupDNSDomain{}}, nil
	}
	metaBytes, err := c.Dump(ctx, metaSnap, "metadata.json")
	if err != nil {
		return nil, bkInternal("read metadata.json", err)
	}
	var meta backup.AccountMetadata
	if err := json.Unmarshal(metaBytes, &meta); err != nil {
		return nil, bkInternal(fmt.Sprintf("parse metadata.json: %v", err), err)
	}

	want := map[string]bool{}
	for _, n := range p.DomainNames {
		want[n] = true
	}
	out := backupDNSReadResult{Domains: []backupDNSDomain{}}
	for _, d := range meta.Domains {
		if !want[d.Name] {
			continue
		}
		out.Domains = append(out.Domains, backupDNSDomain{
			Name:       d.Name,
			DNSRecords: d.DNSRecords,
		})
	}
	return out, nil
}

func init() {
	Default.Register("backup.dns_read", backupDNSReadHandler)
}
