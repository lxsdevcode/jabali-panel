package apps

// InvoiceShelf is the descriptor for InvoiceShelf (GH #631) — a Laravel
// invoicing/billing app (the maintained fork of Crater). RequiresDB=true:
// the agent provisions a MariaDB database, writes .env, runs the Laravel
// migrations + seeders, and creates the operator's admin + company through
// the app's own seeder path (no web wizard) — then marks the onboarding
// complete so the site opens straight at the login page.
//
// admin_username is omitted: InvoiceShelf logs in by EMAIL, so the schema
// asks only for admin_email + optional admin_password.
var InvoiceShelf = App{
	Name:                 "invoiceshelf",
	DisplayName:          "InvoiceShelf",
	Icon:                 "FileTextOutlined",
	Description:          "Self-hosted invoicing and billing for freelancers and small businesses — estimates, invoices, recurring billing, multi-company. The maintained fork of Crater.",
	Tags:                 []string{"Business", "Invoicing"},
	DefaultSubdirectory:  "invoices",
	RequiresDB:           true,
	SupportedPHPVersions: nil,
	AgentInstallCmd:      "app.install",
	AgentDeleteCmd:       "app.delete",
	AgentCloneCmd:        "",
	InstallParamSchema: map[string]ParamSpec{
		"site_title": {
			Type:        "string",
			Required:    true,
			Default:     "InvoiceShelf",
			Description: "Company name shown in the app and on invoices.",
		},
		"admin_email": {
			Type:        "email",
			Required:    true,
			Description: "Administrator email — this is your login (InvoiceShelf authenticates by email, not username).",
		},
		"admin_password": {
			Type:        "password",
			Required:    false,
			Description: "Initial admin password. Leave blank to have one generated and shown once (minimum 8 characters).",
		},
		// GH #1042: the stock seeder hardcodes Kuwaiti Dinar (Crater
		// heritage) and the install skips the onboarding wizard where a
		// currency would normally be chosen, so KWD stuck for every
		// install. Values mirror the app's own seeded currencies table
		// (81 rows, deduped to 78 codes).
		"currency": {
			Type:        "enum",
			Required:    false,
			Default:     "USD",
			Values:      []string{"AED", "ANG", "ARS", "AUD", "AWG", "BAM", "BDT", "BGN", "BRL", "CAD", "CHF", "CLP", "CNY", "COP", "CRC", "CZK", "DKK", "DOP", "DZD", "EGP", "EUR", "GBP", "GHS", "GTQ", "HKD", "HRK", "IDR", "ILS", "INR", "IQD", "JMD", "JPY", "KES", "KGS", "KWD", "LKR", "LYD", "MAD", "MKD", "MOP", "MVR", "MXN", "MYR", "MZN", "NAD", "NGN", "NOK", "NPR", "NZD", "OMR", "PEN", "PHP", "PKR", "PLN", "PYG", "QAR", "RON", "RSD", "RUB", "RWF", "SAR", "SEK", "SGD", "THB", "TMT", "TND", "TRY", "TTD", "TWD", "TZS", "UAH", "USD", "UYU", "VND", "XAF", "XCD", "XOF", "ZAR"},
			Description: "Company currency for invoices and estimates.",
		},
	},
}
