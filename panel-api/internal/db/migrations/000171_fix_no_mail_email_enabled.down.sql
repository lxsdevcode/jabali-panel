-- Irreversible data correction (GH #216): re-deriving email_enabled from
-- mail_provider can't be undone without knowing each row's prior (buggy)
-- value. No-op down — the corrected state is the intended one.
SELECT 1;
