package models

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"strings"
)

// DNS record-type permissions for non-admin tenants (Gitea #466, ADR-0150).
//
// A server-wide matrix controlling which DNS record TYPES a regular user may
// create / edit / delete on their own zones. Enforced authoritatively in
// panel-api; admins always bypass it. NS and SOA are never user-manageable
// (zone infrastructure) and are never represented in the matrix.

// UserManageableDNSTypes is the closed set of record types the matrix governs.
// NS/SOA are deliberately absent — they stay admin-only / Managed.
var UserManageableDNSTypes = []string{"A", "AAAA", "CNAME", "MX", "TXT", "SRV", "CAA"}

// RecordPerm is the per-type create/edit/delete triple.
type RecordPerm struct {
	Create bool `json:"create"`
	Edit   bool `json:"edit"`
	Delete bool `json:"delete"`
}

// DNSUserRecordPolicy maps an UPPERCASE record type to its permissions.
type DNSUserRecordPolicy map[string]RecordPerm

// DefaultDNSUserRecordPolicy is the policy assumed when none is stored: every
// user-manageable type fully permitted. This preserves pre-feature behavior so
// an upgrade never silently strips a tenant's ability to manage their own
// records — admins opt INTO restriction via a preset.
func DefaultDNSUserRecordPolicy() DNSUserRecordPolicy {
	p := make(DNSUserRecordPolicy, len(UserManageableDNSTypes))
	for _, t := range UserManageableDNSTypes {
		p[t] = RecordPerm{Create: true, Edit: true, Delete: true}
	}
	return p
}

// DNSPolicyPreset resolves a named preset to a concrete matrix. ok=false for an
// unknown name. Presets:
//   - "permissive":   every manageable type fully permitted.
//   - "hosting-safe": permissive except CAA (certificate-issuance policy) locked.
//   - "locked-down":  only A/AAAA/CNAME permitted; everything else denied.
func DNSPolicyPreset(name string) (DNSUserRecordPolicy, bool) {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "permissive":
		return DefaultDNSUserRecordPolicy(), true
	case "hosting-safe":
		p := DefaultDNSUserRecordPolicy()
		p["CAA"] = RecordPerm{}
		return p, true
	case "locked-down":
		p := make(DNSUserRecordPolicy, len(UserManageableDNSTypes))
		for _, t := range UserManageableDNSTypes {
			p[t] = RecordPerm{}
		}
		p["A"] = RecordPerm{Create: true, Edit: true, Delete: true}
		p["AAAA"] = RecordPerm{Create: true, Edit: true, Delete: true}
		p["CNAME"] = RecordPerm{Create: true, Edit: true, Delete: true}
		return p, true
	}
	return nil, false
}

// Allows reports whether a non-admin user may perform op ("create"|"edit"|
// "delete") on recordType. Fail-closed: NS/SOA are always denied, an unknown
// type key is denied, and an unknown op is denied. (Whole-policy emptiness is
// handled by the caller, which substitutes the default.)
func (p DNSUserRecordPolicy) Allows(recordType, op string) bool {
	t := strings.ToUpper(strings.TrimSpace(recordType))
	if t == "NS" || t == "SOA" {
		return false
	}
	perm, ok := p[t]
	if !ok {
		return false
	}
	switch op {
	case "create":
		return perm.Create
	case "edit":
		return perm.Edit
	case "delete":
		return perm.Delete
	}
	return false
}

// Sanitize drops keys that are not user-manageable (NS/SOA, unknown types) and
// upper-cases keys, so a PUT cannot smuggle a policy for an admin-only type.
func (p DNSUserRecordPolicy) Sanitize() DNSUserRecordPolicy {
	allowed := make(map[string]bool, len(UserManageableDNSTypes))
	for _, t := range UserManageableDNSTypes {
		allowed[t] = true
	}
	out := make(DNSUserRecordPolicy, len(p))
	for k, v := range p {
		key := strings.ToUpper(strings.TrimSpace(k))
		if allowed[key] {
			out[key] = v
		}
	}
	return out
}

// Value implements driver.Valuer — stored as a JSON string. A nil/empty map is
// stored as "{}".
func (p DNSUserRecordPolicy) Value() (driver.Value, error) {
	if len(p) == 0 {
		return "{}", nil
	}
	b, err := json.Marshal(p)
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

// Scan implements sql.Scanner for JSON stored in a TEXT/JSON column.
func (p *DNSUserRecordPolicy) Scan(src any) error {
	if src == nil {
		*p = DNSUserRecordPolicy{}
		return nil
	}
	var b []byte
	switch v := src.(type) {
	case []byte:
		b = v
	case string:
		b = []byte(v)
	default:
		return fmt.Errorf("DNSUserRecordPolicy: unsupported scan type %T", src)
	}
	if len(b) == 0 {
		*p = DNSUserRecordPolicy{}
		return nil
	}
	m := DNSUserRecordPolicy{}
	if err := json.Unmarshal(b, &m); err != nil {
		return fmt.Errorf("DNSUserRecordPolicy: %w", err)
	}
	*p = m
	return nil
}
