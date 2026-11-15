CREATE TABLE server_config_state (
    server_id INTEGER PRIMARY KEY REFERENCES servers(id) ON DELETE CASCADE,
    desired_revision INTEGER NOT NULL DEFAULT 1,
    applied_revision INTEGER NOT NULL DEFAULT 0,
    status TEXT NOT NULL DEFAULT 'queued' CHECK (status IN ('queued','applying','applied','failed')),
    error TEXT NOT NULL DEFAULT '',
    updated_at TEXT NOT NULL DEFAULT (datetime('now'))
);
INSERT INTO server_config_state(server_id) SELECT id FROM servers;

CREATE TRIGGER server_config_insert AFTER INSERT ON servers BEGIN
    INSERT INTO server_config_state(server_id) VALUES (NEW.id);
END;
CREATE TRIGGER server_config_http2 AFTER UPDATE OF http2_enabled ON servers
WHEN OLD.http2_enabled != NEW.http2_enabled BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=NEW.id;
END;

CREATE TRIGGER proxy_hosts_config_insert AFTER INSERT ON proxy_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=NEW.server_id;
END;

CREATE TRIGGER proxy_hosts_config_update AFTER UPDATE ON proxy_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (OLD.server_id, NEW.server_id);
END;

CREATE TRIGGER proxy_hosts_config_delete AFTER DELETE ON proxy_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=OLD.server_id;
END;

CREATE TRIGGER redirection_hosts_config_insert AFTER INSERT ON redirection_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=NEW.server_id;
END;

CREATE TRIGGER redirection_hosts_config_update AFTER UPDATE ON redirection_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (OLD.server_id, NEW.server_id);
END;

CREATE TRIGGER redirection_hosts_config_delete AFTER DELETE ON redirection_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=OLD.server_id;
END;

CREATE TRIGGER dead_hosts_config_insert AFTER INSERT ON dead_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=NEW.server_id;
END;

CREATE TRIGGER dead_hosts_config_update AFTER UPDATE ON dead_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (OLD.server_id, NEW.server_id);
END;

CREATE TRIGGER dead_hosts_config_delete AFTER DELETE ON dead_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=OLD.server_id;
END;

CREATE TRIGGER stream_hosts_config_insert AFTER INSERT ON stream_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=NEW.server_id;
END;

CREATE TRIGGER stream_hosts_config_update AFTER UPDATE ON stream_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (OLD.server_id, NEW.server_id);
END;

CREATE TRIGGER stream_hosts_config_delete AFTER DELETE ON stream_hosts BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id=OLD.server_id;
END;

CREATE TRIGGER certificates_config_insert AFTER INSERT ON certificates BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now');
END;

CREATE TRIGGER certificates_config_update AFTER UPDATE ON certificates BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now');
END;

CREATE TRIGGER certificates_config_delete AFTER DELETE ON certificates BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now');
END;

CREATE TRIGGER access_lists_config_insert AFTER INSERT ON access_lists BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (SELECT server_id FROM proxy_hosts WHERE access_list_id IN (NEW.id) UNION SELECT server_id FROM redirection_hosts WHERE access_list_id IN (NEW.id) UNION SELECT server_id FROM dead_hosts WHERE access_list_id IN (NEW.id));
END;

CREATE TRIGGER access_lists_config_update AFTER UPDATE ON access_lists BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (SELECT server_id FROM proxy_hosts WHERE access_list_id IN (OLD.id, NEW.id) UNION SELECT server_id FROM redirection_hosts WHERE access_list_id IN (OLD.id, NEW.id) UNION SELECT server_id FROM dead_hosts WHERE access_list_id IN (OLD.id, NEW.id));
END;

CREATE TRIGGER access_lists_config_delete AFTER DELETE ON access_lists BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (SELECT server_id FROM proxy_hosts WHERE access_list_id IN (OLD.id) UNION SELECT server_id FROM redirection_hosts WHERE access_list_id IN (OLD.id) UNION SELECT server_id FROM dead_hosts WHERE access_list_id IN (OLD.id));
END;

CREATE TRIGGER access_list_clients_config_insert AFTER INSERT ON access_list_clients BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (SELECT server_id FROM proxy_hosts WHERE access_list_id IN (NEW.access_list_id) UNION SELECT server_id FROM redirection_hosts WHERE access_list_id IN (NEW.access_list_id) UNION SELECT server_id FROM dead_hosts WHERE access_list_id IN (NEW.access_list_id));
END;

CREATE TRIGGER access_list_clients_config_update AFTER UPDATE ON access_list_clients BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (SELECT server_id FROM proxy_hosts WHERE access_list_id IN (OLD.access_list_id, NEW.access_list_id) UNION SELECT server_id FROM redirection_hosts WHERE access_list_id IN (OLD.access_list_id, NEW.access_list_id) UNION SELECT server_id FROM dead_hosts WHERE access_list_id IN (OLD.access_list_id, NEW.access_list_id));
END;

CREATE TRIGGER access_list_clients_config_delete AFTER DELETE ON access_list_clients BEGIN
    UPDATE server_config_state SET desired_revision=desired_revision+1, status='queued', error='', updated_at=datetime('now') WHERE server_id IN (SELECT server_id FROM proxy_hosts WHERE access_list_id IN (OLD.access_list_id) UNION SELECT server_id FROM redirection_hosts WHERE access_list_id IN (OLD.access_list_id) UNION SELECT server_id FROM dead_hosts WHERE access_list_id IN (OLD.access_list_id));
END;
