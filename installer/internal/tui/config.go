package tui

import (
	"regexp"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

// configField is one text input on the config screen, plus how it maps to an
// install.sh environment variable and whether it only applies with DNS on.
type configField struct {
	env      string // install.sh env var this sets
	label    string
	ph       string // placeholder
	dnsOnly  bool   // only shown/collected when the dns module is enabled
	required bool
	input    textinput.Model
}

var hostnameRe = regexp.MustCompile(`^([a-zA-Z0-9]([-a-zA-Z0-9]*[a-zA-Z0-9])?\.)+[a-zA-Z]{2,}$`)
var emailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

// newConfigFields builds the ordered config inputs. The panel hostname is
// pre-seedable from the environment (JABALI_HOSTNAME) so a half-headless run
// can still tune modules interactively.
func newConfigFields(seedHostname string) []configField {
	mk := func(env, label, ph, def string) textinput.Model {
		ti := textinput.New()
		ti.Placeholder = ph
		ti.CharLimit = 253
		ti.Prompt = ""
		if def != "" {
			ti.SetValue(def)
		}
		return ti
	}
	return []configField{
		{env: "JABALI_HOSTNAME", label: "Panel hostname (FQDN)", ph: "panel.example.com", required: true, input: mk("JABALI_HOSTNAME", "", "panel.example.com", seedHostname)},
		{env: "JABALI_ADMIN_EMAIL", label: "Admin login email", ph: "admin@example.com", required: true, input: mk("", "", "admin@example.com", "")},
		{env: "JABALI_NS1_NAME", label: "Primary nameserver (DNS)", ph: "ns1.example.com", dnsOnly: true, input: mk("", "", "ns1.example.com", "")},
		{env: "JABALI_NS2_NAME", label: "Secondary nameserver (DNS)", ph: "ns2.example.com", dnsOnly: true, input: mk("", "", "ns2.example.com", "")},
		{env: "JABALI_PHP_VERSIONS", label: "PHP versions (space-separated)", ph: "8.4", input: mk("", "", "8.4", "8.4")},
	}
}

// visibleFields returns the field indexes shown given whether DNS is enabled.
func visibleFields(fields []configField, dnsOn bool) []int {
	var idx []int
	for i, f := range fields {
		if f.dnsOnly && !dnsOn {
			continue
		}
		idx = append(idx, i)
	}
	return idx
}

// validateConfig checks required + format for the visible fields; returns the
// first error message, or "" when valid.
func validateConfig(fields []configField, dnsOn bool) string {
	for _, i := range visibleFields(fields, dnsOn) {
		f := fields[i]
		v := strings.TrimSpace(f.input.Value())
		if f.required && v == "" {
			return f.label + " is required"
		}
		switch f.env {
		case "JABALI_HOSTNAME", "JABALI_NS1_NAME", "JABALI_NS2_NAME":
			if v != "" && !hostnameRe.MatchString(v) {
				return f.label + ": not a valid hostname"
			}
		case "JABALI_ADMIN_EMAIL":
			if v != "" && !emailRe.MatchString(v) {
				return f.label + ": not a valid email"
			}
		}
	}
	return ""
}

// configEnv builds the install.sh env assignments from the visible, non-empty
// fields.
func configEnv(fields []configField, dnsOn bool) []string {
	var env []string
	for _, i := range visibleFields(fields, dnsOn) {
		v := strings.TrimSpace(fields[i].input.Value())
		if v != "" {
			env = append(env, fields[i].env+"="+v)
		}
	}
	return env
}

// updateFocusedField routes a key message to the focused input.
func updateFocusedField(fields []configField, focus int, msg tea.Msg) tea.Cmd {
	var cmd tea.Cmd
	fields[focus].input, cmd = fields[focus].input.Update(msg)
	return cmd
}
