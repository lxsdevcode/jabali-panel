-- M PHP-FPM tiers Step 5: remember the user's chosen Performance Mode on the
-- pool. 'custom' = L2 advanced user edited raw pm.*; else one of the 4 presets.
ALTER TABLE php_pools
  ADD COLUMN performance_mode VARCHAR(24) NOT NULL DEFAULT 'balanced';
