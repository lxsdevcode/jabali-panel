ALTER TABLE users MODIFY username VARCHAR(32) NULL;
ALTER TABLE users ADD UNIQUE INDEX ux_users_email (email);
