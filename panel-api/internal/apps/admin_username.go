package apps

// adminUsernamePattern is a conservative subset every username-login app
// accepts (alnum start, then alnum + underscore/hyphen, 2-32 chars) OR empty.
// No dot — Flarum's installer rejects it. Blank is allowed so "leave it to a
// random one" passes validation; the per-app agent installer does any stricter
// app-specific checks. (GH #228)
var adminUsernamePattern = `^([a-zA-Z0-9][a-zA-Z0-9_-]{1,31})?$`

// AdminUsernameParam is the optional admin-username field shared by apps that
// log in by username. Leave it blank and the API generates a random username
// (the historical default); fill it to choose one. Email-login apps (ITFlow,
// PrestaShop) don't use it. (GH #228)
var AdminUsernameParam = ParamSpec{
	Type:        "string",
	Required:    false,
	Pattern:     &adminUsernamePattern,
	Description: "Administrator username. Leave blank for a random one.",
}
