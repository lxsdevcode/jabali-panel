-- Server-wide CrowdSec sensitivity preset. Three levels collapsed into
-- a single enum the operator picks from Security → CrowdSec:
--   relaxed:  ssh-bf 15/60s, ban 30m, AppSec anomaly threshold 10
--             — for ops doing legitimate noisy work from changing IPs
--   balanced: ssh-bf 5/30s, ban 4h, anomaly 5 (CrowdSec + CRS defaults)
--   strict:   ssh-bf 3/30s, ban 24h, anomaly 5 — for paranoid posture
--
-- 'balanced' default matches existing host behavior so the column add
-- is a no-op for installs that don't visit the new toggle.
ALTER TABLE server_settings
  ADD COLUMN crowdsec_sensitivity ENUM('relaxed','balanced','strict')
    NOT NULL DEFAULT 'balanced'
  AFTER appsec_geoblock_countries;
