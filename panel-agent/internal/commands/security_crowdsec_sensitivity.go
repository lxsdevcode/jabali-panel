package commands

// security.crowdsec.sensitivity.apply — write three config files that
// implement the three sensitivity presets the panel UI exposes:
//
//   relaxed  — SSH brute-force 15 fails / 60s, ban 30m, anomaly 10
//   balanced — CrowdSec + CRS defaults (5/30s, 4h, 5)
//   strict   — 3/30s, 24h, 5
//
// Each preset writes (or removes) three jabali-owned files:
//
//   /etc/crowdsec/scenarios/jabali-ssh-bf.yaml    (custom scenario)
//   /etc/crowdsec/appsec-rules/jabali-anomaly.yaml (anomaly threshold)
//   /etc/crowdsec/profiles.d/jabali-ban-duration.yaml (ban window)
//
// Balanced means "stop overriding": we remove all three jabali files
// so CrowdSec falls back to upstream defaults. The agent never edits
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
	sensSSHBfPath   = "/etc/crowdsec/scenarios/jabali-ssh-bf.yaml"
	sensAnomalyPath = "/etc/crowdsec/appsec-rules/jabali-anomaly-threshold.yaml"
	sensProfilePath = "/etc/crowdsec/profiles.d/jabali-ban-duration.yaml"
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
	for _, path := range []string{sensSSHBfPath, sensAnomalyPath, sensProfilePath} {
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
	case "strict":
		if err := writeSensitivityFile(sensSSHBfPath, sshBfYAML(3, "30s", "24h")); err != nil {
			return nil, csInternal("write ssh-bf scenario", err)
		}
		if err := writeSensitivityFile(sensProfilePath, profileYAML("24h")); err != nil {
			return nil, csInternal("write profile", err)
		}
		// anomaly threshold stays at upstream default 5 for strict.
	case "balanced":
		// All overrides removed above; nothing to write.
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

func init() {
	Default.Register("security.crowdsec.sensitivity.apply", sensitivityApplyHandler)
}
