-- Keep the transport credential in step with API token and owner revocation.
CREATE TRIGGER api_tokens_revoke_agent
AFTER UPDATE OF revoked_at ON api_tokens
WHEN OLD.revoked_at IS NULL AND NEW.revoked_at IS NOT NULL AND NEW.server_id IS NOT NULL
BEGIN
    UPDATE servers SET agent_token_hash='' WHERE id=NEW.server_id;
END;
UPDATE servers SET agent_token_hash=''
WHERE id IN (SELECT server_id FROM api_tokens WHERE server_id IS NOT NULL AND revoked_at IS NOT NULL)
  AND NOT EXISTS (SELECT 1 FROM api_tokens WHERE server_id=servers.id AND revoked_at IS NULL);
