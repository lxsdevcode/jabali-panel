-- GH #348: per-group "internal delivery only" toggle. When set, the group
-- address accepts mail only from senders in the same domain; external senders
-- are rejected (enforced by a delivery Sieve on the group principal, applied by
-- the agent's mailgroup.apply). Default 0 = accept mail from anywhere.
ALTER TABLE mail_groups ADD COLUMN internal_only BOOLEAN NOT NULL DEFAULT 0;
