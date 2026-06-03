# Vendored from bu5hm4nn/crowdsec-stalwart

Source: <https://github.com/bu5hm4nn/crowdsec-stalwart>
License: MIT (see `LICENSE`)
Forked: 2026-06-03
Forked-at-commit: HEAD of `main` as of 2026-06-03 (upstream had 2 commits total, no releases)
Tested-against: Stalwart 0.16.6, CrowdSec 1.7.8

## Why we vendor

The upstream repo has 2 commits, 1 star, 0 releases — high abandonment
risk. Per ADR-0115 we own the parser regex and scenarios outright
rather than pin-and-hope.

## Changes vs. upstream

### Parser (`parsers/s01-parse/jabali-stalwart-logs.yaml`)
- Renamed `stalwart-logs` → `jabali/stalwart-logs` to coexist with any
  hub-installed parser of the same name.
- Filter changed from `evt.Parsed.program == 'stalwart'` (file source +
  syslog stage assumption) to `evt.Line.Labels.type == 'stalwart'` so
  the parser keys off the jabali acquis label.
- Grok now applies to `Line.Raw` directly — no intermediate syslog
  parser stage required.
- Protocol mapping rewritten for jabali listener IDs
  (`smtp-25`, `smtp-submissions-465`, `smtp-submission-587`, `imaps`,
  `internal-loopback`) instead of upstream's sample names (`smtp`,
  `submission`, `submissions`).

### Acquis (`acquis.d/jabali-stalwart.yaml`)
- Switched from `source: file` + `filenames: /logs/stalwart/stalwart.log.*`
  (assumes Docker volume mount) to `source: journalctl` keyed on
  `_SYSTEMD_UNIT=jabali-stalwart.service`. Stalwart 0.16 Log tracer
  silently no-ops even with AppArmor widened; Stdout + journal is the
  only ingest path we've confirmed actually emits structured lines
  (live-verified on puzzle 2026-06-03).

### Scenarios (`scenarios/jabali-stalwart-*.yaml`)
- Names prefixed with `jabali/` (e.g. `jabali/stalwart-auth-bf`)
  so they sit alongside the other jabali-namespaced scenarios in
  `cscli alerts list`.
- Bodies otherwise unchanged from upstream — capacity / leakspeed /
  blackhole get overwritten at runtime by the Sensitivity preset
  writer (`security.crowdsec.sensitivity.apply` in panel-agent) in
  PR #181.

### Test samples (`test/samples/`)
- Unchanged from upstream. Useful for `cscli explain` regression
  testing after a Stalwart minor-version bump.

## Pulling in upstream fixes

When the upstream repo updates, diff each file individually and
apply the relevant change manually. Do NOT auto-merge — upstream is
under one maintainer and could change format in ways that break
the jabali wiring above.

```sh
cd /tmp && git clone https://github.com/bu5hm4nn/crowdsec-stalwart u-cs
diff -u u-cs/parsers/s01-parse/stalwart-logs.yaml \
        install/crowdsec/stalwart/parsers/s01-parse/jabali-stalwart-logs.yaml
```
