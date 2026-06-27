package main

import (
	"context"
	"time"

	"git.linux-hosting.co.il/shukivaknin/jabali2/panel-api/internal/repository"
)

// releaseChannelOrDefault reads the operator-selected release channel from
// server_settings (GH #445). Best-effort: any config/DB problem returns
// "development" — the safe default that preserves the historical
// reset-to-origin/main behavior, so a DB hiccup never strands `jabali update`
// trying to resolve a stable tag.
func releaseChannelOrDefault() string {
	const fallback = "development"
	if err := initConfig(); err != nil {
		return fallback
	}
	if err := initDB(); err != nil {
		return fallback
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	s, err := repository.NewServerSettingsRepository(sharedDB).Get(ctx)
	if err != nil || s == nil {
		return fallback
	}
	if s.ReleaseChannel == "stable" {
		return "stable"
	}
	return fallback
}
