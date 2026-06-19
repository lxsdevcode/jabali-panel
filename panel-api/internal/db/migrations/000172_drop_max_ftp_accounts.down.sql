ALTER TABLE hosting_packages
  ADD COLUMN max_ftp_accounts INT UNSIGNED NOT NULL DEFAULT 0 AFTER max_databases;
