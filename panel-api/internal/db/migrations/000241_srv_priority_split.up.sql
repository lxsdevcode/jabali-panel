-- GH #813 follow-up: repair SRV rows written with the priority in BOTH the
-- content string and the priority column.
--
-- PowerDNS's gmysql backend prepends the prio column to the content for SRV,
-- so "0 1 465 mail.example.com" was assembled into a five-field record that
-- pdns refuses to serve — every autodiscovery SRV returned an empty answer.
-- The generator fix stops NEW rows being written that way; it does not touch
-- rows that already exist, because syncEmailDNS skips any (name, type) pair
-- already present (hasExistingM6Record). Without this migration every domain
-- with email already enabled stays broken forever.
--
-- The predicate matches exactly the broken shape: four space-separated fields
-- whose first three are numeric. A repaired row is three fields and no longer
-- matches, so re-running this is a no-op — which also makes it safe on a host
-- that already had rows fixed by hand.
UPDATE dns_records
SET priority = CAST(SUBSTRING_INDEX(content, ' ', 1) AS UNSIGNED),
    content  = TRIM(SUBSTRING(content, LOCATE(' ', content) + 1))
WHERE type = 'SRV'
  AND content REGEXP '^[0-9]+ [0-9]+ [0-9]+ [^ ]+$';
