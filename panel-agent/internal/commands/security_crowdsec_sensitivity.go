package commands

// security.crowdsec.sensitivity.apply — write the jabali-owned
// CrowdSec config files that implement the three sensitivity presets
// the panel UI exposes:
//
//   relaxed  — SSH-bf 15/60s ban 30m; panel-login 15/60s; recovery 10/300s;
//              whoami 50/60s; AppSec anomaly 10; profile ban 30m.
//   balanced — SSH-bf upstream default (5/30s 4h); panel-login 5/60s;
//              recovery 3/300s; whoami 20/60s; AppSec/profile upstream.
//   strict   — SSH-bf 3/30s ban 24h; panel-login 3/60s; recovery 1/300s;
//              whoami 10/60s; anomaly stays at 5; profile ban 24h.
//
// Per-preset files touched:
//
//   /etc/crowdsec/scenarios/jabali-ssh-bf.yaml           SSH brute-force
//   /etc/crowdsec/scenarios/jabali-panel-login-bf.yaml   Kratos /self-service/login
//   /etc/crowdsec/scenarios/jabali-panel-recovery-bf.yaml  Kratos /self-service/recovery
//   /etc/crowdsec/scenarios/jabali-panel-whoami-probe.yaml /sessions/whoami unauth burst
//   /etc/crowdsec/appsec-rules/jabali-anomaly.yaml       CRS anomaly threshold
//   /etc/crowdsec/profiles.d/jabali-ban-duration.yaml    default ban duration
//
// The panel scenarios are always written (every preset) because there
// is no upstream hub equivalent to fall back on — install.sh seeds
// them at the balanced thresholds on fresh hosts. The SSH-bf, anomaly,
// and profile drop-ins are still removed for balanced so CrowdSec
// falls back to the upstream defaults. The agent never edits
// hub-installed files directly — drop-ins only.

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"git.linux-hosting.co.il/shukivaknin/jabali2/agentwire"
)

const (
	sensSSHBfPath          = "/etc/crowdsec/scenarios/jabali-ssh-bf.yaml"
	sensAnomalyPath        = "/etc/crowdsec/appsec-rules/jabali-anomaly-threshold.yaml"
	sensProfilePath        = "/etc/crowdsec/profiles.d/jabali-ban-duration.yaml"
	sensPanelLoginBfPath   = "/etc/crowdsec/scenarios/jabali-panel-login-bf.yaml"
	sensPanelRecoveryBf    = "/etc/crowdsec/scenarios/jabali-panel-recovery-bf.yaml"
	sensPanelWhoamiProbe   = "/etc/crowdsec/scenarios/jabali-panel-whoami-probe.yaml"
)

type sensitivityApplyParams struct {
	Level string `json:"level"`
}

type sensitivityApplyResponse struct {
	Level   string `json:"level"`
	Applied bool   `json:"applied"`
}

func sensitivityApplyHandler(ctx context.Context, params json.RawMessage) (any, error) {
	var p sensitivityApplyParams
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: fmt.Sprintf("parse: %v", err)}
	}
	switch p.Level {
	case "relaxed", "balanced", "strict":
	default:
		return nil, &agentwire.AgentError{Code: agentwire.CodeInvalidArgument, Message: "level must be relaxed|balanced|strict"}
	}

	// Always remove first; preset writers below opt-back-in.
	for _, path := range []string{sensSSHBfPath, sensAnomalyPath, sensProfilePath, sensPanelLoginBfPath, sensPanelRecoveryBf, sensPanelWhoamiProbe} {
		_ = os.Remove(path)
	}

	switch p.Level {
	case "relaxed":
		if err := writeSensitivityFile(sensSSHBfPath, sshBfYAML(15, "60s", "30m")); err != nil {
			return nil, csInternal("write ssh-bf scenario", err)
		}
		if err := writeSensitivityFile(sensAnomalyPath, anomalyYAML(10)); err != nil {
			return nil, csInternal("write anomaly rule", err)
		}
		if err := writeSensitivityFile(sensProfilePath, profileYAML("30m")); err != nil {
			return nil, csInternal("write profile", err)
		}
		if err := writeSensitivityFile(sensPanelLoginBfPath, panelLoginBfYAML(15, "60s", "30m")); err != nil {
			return nil, csInternal("write panel-login-bf", err)
		}
		if err := writeSensitivityFile(sensPanelRecoveryBf, panelRecoveryBfYAML(10, "300s", "1h")); err != nil {
			return nil, csInternal("write panel-recovery-bf", err)
		}
		if err := writeSensitivityFile(sensPanelWhoamiProbe, panelWhoamiProbeYAML(50, "60s", "30m")); err != nil {
			return nil, csInternal("write panel-whoami-probe", err)
		}
	case "strict":
		if err := writeSensitivityFile(sensSSHBfPath, sshBfYAML(3, "30s", "24h")); err != nil {
			return nil, csInternal("write ssh-bf scenario", err)
		}
		if err := writeSensitivityFile(sensProfilePath, profileYAML("24h")); err != nil {
			return nil, csInternal("write profile", err)
		}
		// anomaly threshold stays at upstream default 5 for strict.
		if err := writeSensitivityFile(sensPanelLoginBfPath, panelLoginBfYAML(3, "60s", "24h")); err != nil {
			return nil, csInternal("write panel-login-bf", err)
		}
		if err := writeSensitivityFile(sensPanelRecoveryBf, panelRecoveryBfYAML(1, "300s", "24h")); err != nil {
			return nil, csInternal("write panel-recovery-bf", err)
		}
		if err := writeSensitivityFile(sensPanelWhoamiProbe, panelWhoamiProbeYAML(10, "60s", "24h")); err != nil {
			return nil, csInternal("write panel-whoami-probe", err)
		}
	case "balanced":
		// SSH-bf, anomaly, profile fall back to upstream defaults
		// (cleanup loop above removed any prior jabali drop-in). Panel
		// scenarios have no upstream equivalent, so balanced still
		// writes them â just at the conservative threshold matching
		// what install.sh seeds on fresh hosts.
		if err := writeSensitivityFile(sensPanelLoginBfPath, panelLoginBfYAML(5, "60s", "4h")); err != nil {
			return nil, csInternal("write panel-login-bf", err)
		}
		if err := writeSensitivityFile(sensPanelRecoveryBf, panelRecoveryBfYAML(3, "300s", "4h")); err != nil {
			return nil, csInternal("write panel-recovery-bf", err)
		}
		if err := writeSensitivityFile(sensPanelWhoamiProbe, panelWhoamiProbeYAML(20, "60s", "4h")); err != nil {
			return nil, csInternal("write panel-whoami-probe", err)
		}
	}

	// Reload crowdsec so the new scenario / drop-in profile take effect.
	// SIGHUP via systemctl reload — falls back to restart if reload
	// not wired (older packaging).
	if _, err := exec.CommandContext(ctx, "systemctl", "reload", "crowdsec").CombinedOutput(); err != nil {
		if _, err2 := exec.CommandContext(ctx, "systemctl", "restart", "crowdsec").CombinedOutput(); err2 != nil {
			return nil, csInternal("reload/restart crowdsec", fmt.Errorf("reload: %v; restart: %v", err, err2))
		}
	}
	return sensitivityApplyResponse{Level: p.Level, Applied: true}, nil
}

// writeSensitivityFile atomic-writes via tmp+rename. Mode 0644 root:root
// to match what jabali install.sh seeds for sibling scenario files.
func writeSensitivityFile(path, body string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".jabali-cs-sens-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.WriteString(body); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Chmod(0o644); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}

// sshBfYAML produces a leaky-bucket scenario tuned per preset. capacity
// is the bucket size (fails-to-fill), leakspeed is per-event drain
// interval, blackhole is the after-trigger silence window.
func sshBfYAML(capacity int, leakspeed, blackhole string) string {
	return fmt.Sprintf(`# Managed by jabali — Security → CrowdSec → Sensitivity. Do not hand-edit.
type: leaky
name: jabali/ssh-bf-tuned
description: "Jabali-tuned SSH brute-force (sensitivity preset)"
filter: "evt.Meta.log_type == 'ssh_failed-auth'"
leakspeed: %q
capacity: %d
groupby: "evt.Meta.source_ip"
blackhole: %s
labels:
  service: ssh
  type: bruteforce
  remediation: true
`, leakspeed, capacity, blackhole)
}

// anomalyYAML overrides the CRS inbound-anomaly score threshold.
// SecAction phase:1 with a setvar runs before request-evaluation
// rules so the new threshold applies to every subsequent rule.
func anomalyYAML(threshold int) string {
	return fmt.Sprintf(`# Managed by jabali — Security → CrowdSec → Sensitivity. Do not hand-edit.
name: jabali/anomaly-threshold
description: "Raise CRS inbound-anomaly score threshold (sensitivity preset)"
seclang_rules:
 - SecAction "id:900110,phase:1,pass,nolog,t:none,setvar:tx.inbound_anomaly_score_threshold=%d"
`, threshold)
}

// profileYAML overrides the default ban duration. CrowdSec applies
// profiles in order; the jabali drop-in must come BEFORE the default,
// so we land it in profiles.d/ which loads alphabetically + before the
// hub-managed profiles.yaml is consulted.
func profileYAML(duration string) string {
	return fmt.Sprintf(`# Managed by jabali — Security → CrowdSec → Sensitivity. Do not hand-edit.
name: jabali-ip-remediation
filters:
  - Alert.Remediation == true && Alert.GetScope() == "Ip"
decisions:
  - type: ban
    duration: %s
on_success: break
`, duration)
}

// panelLoginBfYAML â burst of POST hits to the Kratos login submit
// endpoint returning 4xx. The SPA mounts Kratos at /.ory so the real
// path observed in the panel access log is /.ory/self-service/login.
// Matching .Path with `contains` keeps the rule resilient to Kratos's
// flow-id query strings (?flow=<id>) and the legacy non-prefixed path.
func panelLoginBfYAML(capacity int, leakspeed, blackhole string) string {
	return fmt.Sprintf(`# Managed by jabali â Security â CrowdSec â Sensitivity. Do not hand-edit.
type: leaky
name: jabali/panel-login-bf
description: "Brute-force on the jabali panel login (Kratos /self-service/login)"
filter: |
  evt.Meta.log_type == 'http_access-log' &&
  evt.Meta.http_path contains '/self-service/login' &&
  evt.Meta.http_verb == 'POST' &&
  evt.Meta.http_status in ['400','401','403','422']
distinct: evt.Meta.source_ip
leakspeed: %q
capacity: %d
groupby: evt.Meta.source_ip
blackhole: %s
labels:
  service: jabali-panel
  type: bruteforce
  remediation: true
`, leakspeed, capacity, blackhole)
}

// panelRecoveryBfYAML â password-recovery flow abuse. Submitting a
// recovery code (POST /self-service/recovery) repeatedly with a wrong
// code is a way to brute-force the OTP. Recovery codes have lower
// entropy than passwords so the threshold is tighter than login-bf.
func panelRecoveryBfYAML(capacity int, leakspeed, blackhole string) string {
	return fmt.Sprintf(`# Managed by jabali â Security â CrowdSec â Sensitivity. Do not hand-edit.
type: leaky
name: jabali/panel-recovery-bf
description: "Brute-force on the jabali panel recovery flow (Kratos /self-service/recovery)"
filter: |
  evt.Meta.log_type == 'http_access-log' &&
  evt.Meta.http_path contains '/self-service/recovery' &&
  evt.Meta.http_verb == 'POST' &&
  evt.Meta.http_status in ['400','401','403','422']
distinct: evt.Meta.source_ip
leakspeed: %q
capacity: %d
groupby: evt.Meta.source_ip
blackhole: %s
labels:
  service: jabali-panel
  type: bruteforce
  remediation: true
`, leakspeed, capacity, blackhole)
}

// panelWhoamiProbeYAML â unauth bursts on /sessions/whoami. The SPA
// polls whoami once per app load to bootstrap the session; bursts of
// 401s from one IP are session-token guessing or unauthenticated
// probing. Threshold is higher than login-bf because legitimate
// page reloads from a logged-out browser still hit this path.
func panelWhoamiProbeYAML(capacity int, leakspeed, blackhole string) string {
	return fmt.Sprintf(`# Managed by jabali â Security â CrowdSec â Sensitivity. Do not hand-edit.
type: leaky
name: jabali/panel-whoami-probe
description: "Unauth whoami burst (session probing on Kratos /sessions/whoami)"
filter: |
  evt.Meta.log_type == 'http_access-log' &&
  evt.Meta.http_path contains '/sessions/whoami' &&
  evt.Meta.http_status == '401'
distinct: evt.Meta.source_ip
leakspeed: %q
capacity: %d
groupby: evt.Meta.source_ip
blackhole: %s
labels:
  service: jabali-panel
  type: probing
  remediation: true
`, leakspeed, capacity, blackhole)
}

func init() {
	Default.Register("security.crowdsec.sensitivity.apply", sensitivityApplyHandler)
}
