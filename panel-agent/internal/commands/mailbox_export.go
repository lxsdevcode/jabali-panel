// mailbox_export.go — export one Stalwart account's messages to a
// cpanel-style Maildir tree via JMAP, the inverse of
// migration.import_mailboxes. Used by backup.mailboxes (ADR-0123) to
// capture per-user message bodies in a multi-tenant-safe, restorable
// form — replacing the whole-store bodies.tar.
//
// Layout written (consumed verbatim by migration.import_mailboxes):
//
//	<destRoot>/mail/<domain>/<local>/{cur,new}/               (INBOX)
//	<destRoot>/mail/<domain>/<local>/.<Sub>/{cur,new}/        (subfolders)
//
// The importer keys "seen" off cur/ vs new/ and reads receivedAt from
// the file mtime, so we encode exactly those two facts: $seen → cur/
// (else new/), and os.Chtimes(file, receivedAt). No Maildir flag string
// is needed.
package commands

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// emailQueryPageSize bounds each Email/query page. Stalwart returns ids
// in one shot for small mailboxes; pagination guards huge ones.
const emailQueryPageSize = 500

// roleToMaildirSubdir maps a Stalwart mailbox role to the Maildir++
// subfolder dir the importer recognizes (inverse of
// maildirSubfolderRole). Empty string = the INBOX root (no subdir).
// A folder with no role falls back to its sanitized display name.
func roleToMaildirSubdir(role, name string) string {
	switch strings.ToLower(role) {
	case "inbox":
		return ""
	case "sent":
		return ".Sent"
	case "drafts":
		return ".Drafts"
	case "junk":
		return ".Junk"
	case "trash":
		return ".Trash"
	case "archive":
		return ".Archive"
	}
	// Custom folder: Maildir++ ".<Name>", with path separators and the
	// reserved dot stripped so we never escape the maildir or create a
	// nested slot the importer can't read.
	clean := strings.ReplaceAll(name, "/", "_")
	clean = strings.ReplaceAll(clean, string(os.PathSeparator), "_")
	clean = strings.TrimLeft(clean, ".")
	if clean == "" {
		return ""
	}
	return "." + clean
}

type jmapMailbox struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Role string `json:"role"`
}

type jmapEmailMeta struct {
	ID         string          `json:"id"`
	BlobID     string          `json:"blobId"`
	Size       int64           `json:"size"`
	ReceivedAt string          `json:"receivedAt"`
	Keywords   map[string]bool `json:"keywords"`
}

// exportMailboxToMaildir writes every message in the account behind
// `email` to a Maildir tree rooted at destRoot. Returns (messages,
// bytes). A not-yet-registered account (no JMAP principal) exports
// nothing and is not an error — there are no messages to capture.
func exportMailboxToMaildir(ctx context.Context, email, destRoot string) (msgs int64, bytes int64, err error) {
	at := strings.LastIndex(email, "@")
	if at <= 0 || at == len(email)-1 {
		return 0, 0, fmt.Errorf("exportMailboxToMaildir: malformed email %q", email)
	}
	local, domain := email[:at], email[at+1:]

	accountID, err := accountIDByEmail(ctx, email)
	if err != nil {
		return 0, 0, fmt.Errorf("resolve account %q: %w", email, err)
	}
	if accountID == "" {
		return 0, 0, nil // not registered → nothing to export
	}

	var mbResp struct {
		List []jmapMailbox `json:"list"`
	}
	mbArgs := map[string]any{
		"accountId":  accountID,
		"ids":        nil,
		"properties": []string{"id", "name", "role"},
	}
	if err := jmapCallWith(ctx, "urn:ietf:params:jmap:mail", "Mailbox/get", mbArgs, &mbResp); err != nil {
		return 0, 0, fmt.Errorf("Mailbox/get %q: %w", email, err)
	}

	// destRoot is the mail staging dir; the importer's src_mail_dir
	// points here and walks <domain>/<local> subdirs.
	mailRoot := filepath.Join(destRoot, domain, local)
	for _, mb := range mbResp.List {
		sub := roleToMaildirSubdir(mb.Role, mb.Name)
		maildir := mailRoot
		if sub != "" {
			maildir = filepath.Join(mailRoot, sub)
		}
		n, b, ferr := exportFolder(ctx, accountID, mb.ID, maildir)
		if ferr != nil {
			return msgs, bytes, fmt.Errorf("export folder %q (%s): %w", mb.Name, email, ferr)
		}
		msgs += n
		bytes += b
	}
	return msgs, bytes, nil
}

// exportFolder paginates a single mailbox's messages and writes each to
// the maildir's cur/ (seen) or new/ (unseen) slot.
func exportFolder(ctx context.Context, accountID, mailboxID, maildir string) (msgs int64, bytes int64, err error) {
	position := 0
	for {
		var qResp struct {
			IDs []string `json:"ids"`
		}
		qArgs := map[string]any{
			"accountId": accountID,
			"filter":    map[string]any{"inMailbox": mailboxID},
			"position":  position,
			"limit":     emailQueryPageSize,
		}
		if err := jmapCallWith(ctx, "urn:ietf:params:jmap:mail", "Email/query", qArgs, &qResp); err != nil {
			return msgs, bytes, fmt.Errorf("Email/query: %w", err)
		}
		if len(qResp.IDs) == 0 {
			return msgs, bytes, nil
		}

		var gResp struct {
			List []jmapEmailMeta `json:"list"`
		}
		gArgs := map[string]any{
			"accountId":  accountID,
			"ids":        qResp.IDs,
			"properties": []string{"id", "blobId", "size", "receivedAt", "keywords"},
		}
		if err := jmapCallWith(ctx, "urn:ietf:params:jmap:mail", "Email/get", gArgs, &gResp); err != nil {
			return msgs, bytes, fmt.Errorf("Email/get: %w", err)
		}

		for _, m := range gResp.List {
			if m.Size > migrationMailboxMessageCap {
				continue // skip oversize, mirror importer
			}
			seen := m.Keywords["$seen"]
			n, werr := writeMaildirMessage(ctx, accountID, m, maildir, seen)
			if werr != nil {
				return msgs, bytes, werr
			}
			msgs++
			bytes += n
		}

		if len(qResp.IDs) < emailQueryPageSize {
			return msgs, bytes, nil
		}
		position += emailQueryPageSize
	}
}

// writeMaildirMessage downloads the RFC822 blob and writes it to the
// cur/ (seen) or new/ (unseen) slot, stamping the file mtime with
// receivedAt so the importer recovers it.
func writeMaildirMessage(ctx context.Context, accountID string, m jmapEmailMeta, maildir string, seen bool) (int64, error) {
	slot := "new"
	if seen {
		slot = "cur"
	}
	dir := filepath.Join(maildir, slot)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return 0, fmt.Errorf("mkdir %s: %w", dir, err)
	}
	// Filename is irrelevant to the importer (it reads slot + mtime),
	// but must be unique within the slot. JMAP email id is unique per
	// account; tag with the size for a Maildir-ish shape.
	fname := m.ID + ".jabali," + strconv.FormatInt(m.Size, 10)
	dst := filepath.Join(dir, fname)

	n, err := downloadBlob(ctx, accountID, m.BlobID, dst)
	if err != nil {
		return 0, fmt.Errorf("download %s: %w", m.ID, err)
	}
	if rcv, perr := time.Parse(time.RFC3339, m.ReceivedAt); perr == nil {
		_ = os.Chtimes(dst, rcv, rcv)
	}
	return n, nil
}

// downloadBlob streams a Stalwart blob (an RFC822 message) to `dst`.
// Mirrors uploadBlob in migration_import_mailboxes.go: same admin
// Basic-auth, same HTTP client, same loopback admin URL.
func downloadBlob(ctx context.Context, accountID, blobID, dst string) (int64, error) {
	url := stalwartAdminURLFunc() + "/jmap/download/" + accountID + "/" + blobID + "/message.eml"
	subctx, cancel := context.WithTimeout(ctx, migrationMailboxJMAPCallTimeout)
	defer cancel()
	req, err := http.NewRequestWithContext(subctx, http.MethodGet, url, nil)
	if err != nil {
		return 0, err
	}
	token, err := stalwartAdminTokenFunc()
	if err != nil {
		return 0, err
	}
	req.SetBasicAuth(jmapAdminUser, token)

	resp, err := stalwartHTTPClientFunc().Do(req)
	if err != nil {
		return 0, fmt.Errorf("get download: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		return 0, fmt.Errorf("download HTTP %d: %s", resp.StatusCode, string(body))
	}

	f, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0o600)
	if err != nil {
		return 0, err
	}
	defer f.Close()
	n, err := io.Copy(f, io.LimitReader(resp.Body, migrationMailboxMessageCap+1))
	if err != nil {
		return 0, err
	}
	return n, nil
}
