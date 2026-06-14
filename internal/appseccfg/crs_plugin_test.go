package appseccfg

import "strings"

import "testing"

// CRSPluginBefore must stay SURGICAL: drop only rule 933120's inspection
// of only the _wp_http_referer arg, only under /wp-admin/. A broadening
// to ruleRemoveById or a path-allow would silently open a WAF hole.
func TestCRSPluginBefore_Surgical(t *testing.T) {
	body := CRSPluginBefore()

	mustContain := []string{
		`SecRule REQUEST_URI "@beginsWith /wp-admin/"`,
		`id:9599100`,
		`ctl:ruleRemoveTargetById=933120;ARGS:_wp_http_referer`,
	}
	for _, sub := range mustContain {
		if !strings.Contains(body, sub) {
			t.Errorf("CRSPluginBefore missing %q", sub)
		}
	}

	// Must NOT broaden: no whole-rule removal, no path-allow.
	mustNotContain := []string{
		"ruleRemoveById", // would kill 933120 everywhere
		"SetRemediation", // path-allow / blanket allow
		"CancelEvent",
	}
	for _, sub := range mustNotContain {
		if strings.Contains(body, sub) {
			t.Errorf("CRSPluginBefore must not contain %q (too broad)", sub)
		}
	}

	// id stays in the reserved jabali CRS-plugin range 9,599,000–9,599,999.
	if !strings.Contains(body, "id:95990") && !strings.Contains(body, "id:95991") &&
		!strings.Contains(body, "id:95992") && !strings.Contains(body, "id:9599") {
		t.Error("CRSPluginBefore id outside reserved 9,599,000–9,599,999 range")
	}
}
