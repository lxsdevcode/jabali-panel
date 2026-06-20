package apps

// ITFlow is the descriptor for ITFlow — an open-source MSP ERP (clients,
// ticketing, invoicing, documentation). RequiresDB=true.
//
// First git-clone app: ITFlow ships only via git (it self-updates along
// `master` via git pull from its own admin UI), so the agent clones the
// repo and runs ITFlow's headless `scripts/setup_cli.php` (which writes
// config.php, imports db.sql, and creates the first admin). See the
// agent's itflow_install.go.
//
// ITFlow logs in by EMAIL (not a username); admin_name is the admin's
// display name. Three cron jobs (cron.php daily, mail_queue.php +
// ticket_email_parser.php per-minute) are auto-created at install time by
// the API kicker and torn down on delete.
//
// Clone (duplicate install) is intentionally empty — AgentCloneCmd stays
// blank so the Clone button is hidden. Delete reuses app.delete (a
// dedicated itflow deleter) and the install row's db_id drives the DB drop.
var ITFlow = App{
	Name:                 "itflow",
	DisplayName:          "ITFlow",
	Icon:                 "AppstoreOutlined",
	Description:          "Open-source MSP ERP — client management, ticketing, invoicing, documentation and assets for IT businesses.",
	DefaultSubdirectory:  "",
	RequiresDB:           true,
	EmailLogin:           true,
	RootOnly:             true,
	SupportedPHPVersions: nil,
	AgentInstallCmd:      "app.install",
	AgentDeleteCmd:       "app.delete",
	AgentCloneCmd:        "",
	InstallParamSchema: map[string]ParamSpec{
		"company_name": {
			Type:        "string",
			Required:    true,
			Default:     "My Company",
			Description: "Your company name — seeds ITFlow's first company record.",
		},
		"admin_name": {
			Type:        "string",
			Required:    true,
			Default:     "Administrator",
			Description: "Administrator full name (ITFlow logs in by email; this is the display name).",
		},
		"admin_email": {
			Type:        "email",
			Required:    true,
			Description: "Administrator email — this is the ITFlow login.",
		},
		"admin_password": {
			Type:        "password",
			Required:    false,
			Description: "Initial admin password. Leave blank to have one generated. ITFlow requires ≥8 chars; the generator satisfies that.",
		},
	},
}
