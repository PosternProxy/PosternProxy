CREATE TABLE auth_sessions (
    id TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    refresh_hash TEXT NOT NULL,
    expires_at TEXT NOT NULL
);
CREATE INDEX auth_sessions_user_id ON auth_sessions(user_id);

CREATE TRIGGER users_revoke_credentials
AFTER UPDATE OF password_hash, role, is_active ON users
WHEN OLD.password_hash != NEW.password_hash OR OLD.role != NEW.role OR OLD.is_active != NEW.is_active
BEGIN
    DELETE FROM auth_sessions WHERE user_id = NEW.id;
    UPDATE api_tokens SET revoked_at = datetime('now')
      WHERE created_by = NEW.id AND revoked_at IS NULL;
END;

CREATE TRIGGER users_delete_credentials BEFORE DELETE ON users
BEGIN
    UPDATE api_tokens SET revoked_at = datetime('now')
      WHERE created_by = OLD.id AND revoked_at IS NULL;
END;
