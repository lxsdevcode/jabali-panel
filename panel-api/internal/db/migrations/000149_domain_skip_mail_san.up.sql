-- Per-domain opt-out of auto-added mail/autoconfig SAN entries.
--
-- Background: ADR-0070 SAN auto-provisioning adds mail.<domain> +
-- autoconfig.<domain> to the cert SAN list once email_enabled flips.
-- If the tenant's DNS doesn't have those subdomains, the LE
-- HTTP-01 challenge fails for them and the WHOLE cert stays in
-- pending state. This flag lets the operator say "skip the SAN
-- auto-add for this domain" so the cert covers just <domain> +
-- www.<domain> + (mta-sts if MTASTSEnabled).

ALTER TABLE domains
  ADD COLUMN skip_auto_san TINYINT(1) NOT NULL DEFAULT 0
    AFTER email_enabled;
