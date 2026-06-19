-- Remove the vestigial max_ftp_accounts package field. jabali does not
-- support FTP (SFTP is single-account per hosting user), so the limit was
-- never enforced — only stored and displayed. See GH #219.
ALTER TABLE hosting_packages DROP COLUMN max_ftp_accounts;
