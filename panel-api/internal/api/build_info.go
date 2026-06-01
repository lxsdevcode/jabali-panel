package api

// Build-info vars overridden at link time via `go build -ldflags '-X
// .../api.Commit=<full-sha> -X .../api.BuildTime=<rfc3339>'`. Default
// "unknown" survives untagged dev builds so consumers can format the
// info without nil checks. Version (semver or short SHA) is declared
// alongside in health.go because /health already exposes it.
var (
	// Commit is the full git SHA the binary was built from.
	Commit = "unknown"
	// BuildTime is the RFC3339 timestamp at link time.
	BuildTime = "unknown"
)
