package commands

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

// GH #348: "internal delivery only" for a group address. When enabled, a
// delivery Sieve on the group principal rejects mail whose envelope sender is
// NOT in the group's own domain, so only same-domain senders can reach it.
//
// The reject fires at DELIVERY (post-SMTP-accept), so an external sender gets a
// bounce rather than an SMTP-time 5xx — the message still never reaches the
// group. Validated live on 192.168.100.86 (external sender bounced, same-domain
// delivered).
const (
	jmapCapSieve                = "urn:ietf:params:jmap:sieve"
	groupInternalOnlyScriptName = "jabali-internal-only"
)

// applyGroupInternalOnly installs (internalOnly=true) or removes
// (internalOnly=false) the internal-only delivery Sieve on the group principal.
// Best-effort: the group already exists at apply time, so a Sieve hiccup must
// never fail the apply. Domain is derived from the group address.
func applyGroupInternalOnly(ctx context.Context, groupEmail string, internalOnly bool) {
	acctID, err := accountIDByEmail(ctx, groupEmail)
	if err != nil || acctID == "" {
		return
	}
	if !internalOnly {
		_ = destroyGroupInternalOnlySieve(ctx, acctID)
		return
	}
	at := strings.LastIndex(groupEmail, "@")
	if at < 0 || at+1 >= len(groupEmail) {
		return
	}
	domain := groupEmail[at+1:]
	// Sieve escaping: the domain is a DNS name (validated upstream), no quotes.
	script := fmt.Sprintf(
		"require [\"envelope\",\"reject\"];\n"+
			"if not envelope :domain :is \"from\" \"%s\" {\n"+
			"  reject \"550 5.7.1 This address only accepts mail from within the domain.\";\n"+
			"}\n",
		domain,
	)
	blobID, err := uploadSieveBlob(ctx, acctID, script)
	if err != nil || blobID == "" {
		return
	}
	args := map[string]any{
		"accountId": acctID,
		"create": map[string]any{
			"s": map[string]any{"name": groupInternalOnlyScriptName, "blobId": blobID},
		},
		"onSuccessActivateScript": "#s",
	}
	var result jmapSetResult
	_ = jmapCallWith(ctx, jmapCapSieve, "SieveScript/set", args, &result)
}

// destroyGroupInternalOnlySieve removes the internal-only script by name (idempotent).
func destroyGroupInternalOnlySieve(ctx context.Context, accountID string) error {
	var got struct {
		List []struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		} `json:"list"`
	}
	if err := jmapCallWith(ctx, jmapCapSieve, "SieveScript/get",
		map[string]any{"accountId": accountID}, &got); err != nil {
		return err
	}
	var id string
	for _, s := range got.List {
		if s.Name == groupInternalOnlyScriptName {
			id = s.ID
			break
		}
	}
	if id == "" {
		return nil
	}
	var result jmapSetResult
	return jmapCallWith(ctx, jmapCapSieve, "SieveScript/set",
		map[string]any{"accountId": accountID, "destroy": []string{id}}, &result)
}

// uploadSieveBlob uploads a Sieve script and returns its blob id.
func uploadSieveBlob(ctx context.Context, accountID, script string) (string, error) {
	token, err := stalwartAdminTokenFunc()
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		stalwartAdminURLFunc()+"/jmap/upload/"+accountID, bytes.NewReader([]byte(script)))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/sieve")
	req.SetBasicAuth(jmapAdminUser, token)
	resp, err := stalwartHTTPClientFunc().Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("sieve upload HTTP %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
	if err != nil {
		return "", err
	}
	var out struct {
		BlobID string `json:"blobId"`
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return "", err
	}
	return out.BlobID, nil
}
