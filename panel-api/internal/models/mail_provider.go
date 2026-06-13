package models

// Mail provider enum (GH#181 / ADR-0120). MailProvider on a Domain is the
// single source of truth for the domain's mail posture; EmailEnabled and
// SkipAutoSAN are derived from it via DeriveMailFlags so the two booleans
// can never drift from operator intent.
const (
	MailProviderJabali = "jabali" // Jabali is the MTA (default; today's behavior)
	MailProviderNone   = "none"   // no mail anywhere — no mail records, no mail SANs
	MailProviderM365   = "m365"   // Microsoft 365
	MailProviderGoogle = "google" // Google Workspace
)

// ValidMailProvider reports whether p is a recognised provider value.
func ValidMailProvider(p string) bool {
	switch p {
	case MailProviderJabali, MailProviderNone, MailProviderM365, MailProviderGoogle:
		return true
	default:
		return false
	}
}

// MailProviderIsExternal reports whether the provider hosts mail somewhere
// other than Jabali (M365 / Google) — i.e. Jabali is not the MTA but the
// domain still has mail (so provider DNS records are published).
func MailProviderIsExternal(p string) bool {
	return p == MailProviderM365 || p == MailProviderGoogle
}

// DeriveMailFlags returns the (EmailEnabled, SkipAutoSAN) pair implied by a
// provider. This is the ONLY place those two booleans are computed from
// provider intent — callers must use it on create/edit so the columns the
// reconciler + cert paths read stay consistent with the enum.
//
//	jabali          -> (true,  false)  Jabali MTA + jabali mail SANs on cert
//	none/m365/google-> (false, true)   not the Jabali MTA; no jabali mail SANs
func DeriveMailFlags(provider string) (emailEnabled, skipAutoSAN bool) {
	if provider == MailProviderJabali {
		return true, false
	}
	return false, true
}
