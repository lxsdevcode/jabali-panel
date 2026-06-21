package commands

import "testing"

// A trimmed, realistic .com whois block (ICANN gTLD format).
const whoisComFixture = `Domain Name: EXAMPLE.COM
Registry Domain ID: 2336799_DOMAIN_COM-VRSN
Registrar WHOIS Server: whois.iana.org
Registrar URL: http://res-dom.iana.org
Updated Date: 2024-08-14T07:01:34Z
Creation Date: 1995-08-14T04:00:00Z
Registry Expiry Date: 2025-08-13T04:00:00Z
Registrar: RESERVED-Internet Assigned Numbers Authority
Domain Status: clientDeleteProhibited https://icann.org/epp#clientDeleteProhibited
Domain Status: clientTransferProhibited https://icann.org/epp#clientTransferProhibited
Name Server: A.IANA-SERVERS.NET
Name Server: B.IANA-SERVERS.NET
DNSSEC: signedDelegation
`

func TestParseWhois_GTLD(t *testing.T) {
	r := parseWhois("example.com", whoisComFixture)
	if r.Registrar != "RESERVED-Internet Assigned Numbers Authority" {
		t.Errorf("registrar = %q", r.Registrar)
	}
	if r.CreatedDate != "1995-08-14T04:00:00Z" {
		t.Errorf("created = %q", r.CreatedDate)
	}
	if r.UpdatedDate != "2024-08-14T07:01:34Z" {
		t.Errorf("updated = %q", r.UpdatedDate)
	}
	if r.ExpiryDate != "2025-08-13T04:00:00Z" {
		t.Errorf("expiry = %q", r.ExpiryDate)
	}
	if len(r.NameServers) != 2 || r.NameServers[0] != "a.iana-servers.net" {
		t.Errorf("name servers = %v", r.NameServers)
	}
	if len(r.Statuses) != 2 {
		t.Errorf("statuses = %v", r.Statuses)
	}
	if r.Raw == "" {
		t.Error("raw must always be populated")
	}
}

// An unrecognised / sparse format must still return raw without erroring,
// leaving parsed fields empty (the panel shows raw + DB info).
func TestParseWhois_UnknownFormatKeepsRaw(t *testing.T) {
	raw := "this registry uses a free-form layout\nwith no key: value pairs we know\n"
	r := parseWhois("weird.tld", raw)
	if r.Raw != raw {
		t.Error("raw not preserved verbatim")
	}
	if r.Registrar != "" || r.ExpiryDate != "" || len(r.NameServers) != 0 {
		t.Errorf("expected empty parsed fields, got %+v", r)
	}
}

// .ru/ccTLD-style keys (nserver, paid-till, org) parse too.
func TestParseWhois_CCTLDAliases(t *testing.T) {
	raw := `domain: EXAMPLE.RU
nserver: ns1.example.ru
nserver: ns2.example.ru
state: REGISTERED, DELEGATED
org: Example LLC
registrar: RU-CENTER-RU
created: 2008-07-21T20:00:00Z
paid-till: 2025-08-01T21:00:00Z
`
	r := parseWhois("example.ru", raw)
	if r.Registrar != "RU-CENTER-RU" {
		t.Errorf("registrar = %q", r.Registrar)
	}
	if r.ExpiryDate != "2025-08-01T21:00:00Z" {
		t.Errorf("expiry = %q", r.ExpiryDate)
	}
	if r.Registrant != "Example LLC" {
		t.Errorf("registrant = %q", r.Registrant)
	}
	if len(r.NameServers) != 2 {
		t.Errorf("name servers = %v", r.NameServers)
	}
}
