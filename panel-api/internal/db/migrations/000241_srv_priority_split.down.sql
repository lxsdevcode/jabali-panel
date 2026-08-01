-- Re-inline the priority, restoring the (broken) four-field content. Only
-- touches rows that currently hold the repaired three-field shape, so this is
-- likewise idempotent.
UPDATE dns_records
SET content  = CONCAT(priority, ' ', content),
    priority = 0
WHERE type = 'SRV'
  AND content REGEXP '^[0-9]+ [0-9]+ [^ ]+$';
