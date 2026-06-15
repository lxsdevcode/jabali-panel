-- Admin act-as impersonation grants (GH #183, ADR-0128). A grant lets an
-- authenticated admin operate on a target (non-admin) user's resources via an
-- effective-user override at the authorization layer — NOT a Kratos session
-- swap, so the admin's own session is untouched. Server-side (vs a bare
-- header) so impersonation is revocable + time-boxed; the X-Jabali-Act-As
-- header carries the grant id, and the override only acts when the real Kratos
-- cookie belongs to admin_user_id.
CREATE TABLE impersonation_grants (
    id             CHAR(26)    NOT NULL PRIMARY KEY,
    admin_user_id  CHAR(26)    NOT NULL,
    target_user_id CHAR(26)    NOT NULL,
    created_at     DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
    expires_at     DATETIME(6) NOT NULL,
    ended_at       DATETIME(6) NULL,
    INDEX idx_impersonation_grants_admin (admin_user_id),
    INDEX idx_impersonation_grants_expires (expires_at),
    CONSTRAINT fk_impersonation_grants_admin
        FOREIGN KEY (admin_user_id) REFERENCES users (id) ON DELETE CASCADE,
    CONSTRAINT fk_impersonation_grants_target
        FOREIGN KEY (target_user_id) REFERENCES users (id) ON DELETE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;
