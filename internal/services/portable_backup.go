package services

import (
	"context"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/posternproxy/posternproxy/internal/certstore"
	"github.com/posternproxy/posternproxy/internal/security"
	"golang.org/x/crypto/scrypt"
)

const portableVersion = "2.0"
const maxPortableSize = 50 << 20

type PortableRestoreOptions struct {
	Passphrase       string          `json:"passphrase"`
	ServerMapping    map[int64]int64 `json:"server_mapping"`
	CreateUnenrolled bool            `json:"create_unenrolled"`
}
type portableEnvelope struct {
	Version    string `json:"version"`
	Salt       string `json:"salt"`
	Ciphertext string `json:"ciphertext"`
}
type portableServer struct {
	ID       int64  `json:"id"`
	Name     string `json:"name"`
	Hostname string `json:"hostname"`
	Local    bool   `json:"local"`
}
type portablePayload struct {
	Config       BackupPayload            `json:"config"`
	Servers      []portableServer         `json:"servers"`
	Certificates map[int64]certstore.Pair `json:"certificates"`
}

func (s *BackupService) ConfigureCertificates(store certstore.Store) { s.store = &store }
func portableKey(passphrase string, salt []byte) (key [32]byte, err error) {
	if len(passphrase) < 12 || len(passphrase) > 1024 {
		return key, fmt.Errorf("backup passphrase must contain 12 to 1024 bytes")
	}
	if len(salt) != 32 {
		return key, fmt.Errorf("invalid backup salt")
	}
	derived, err := scrypt.Key([]byte(passphrase), salt, 32768, 8, 1, 32)
	if err != nil {
		return key, fmt.Errorf("derive backup key: %w", err)
	}
	copy(key[:], derived)
	return key, nil
}
func (s *BackupService) ExportPortable(ctx context.Context, passphrase string) ([]byte, error) {
	salt := make([]byte, 32)
	if _, err := rand.Read(salt); err != nil {
		return nil, err
	}
	key, err := portableKey(passphrase, salt)
	if err != nil {
		return nil, err
	}
	config, servers, err := s.exportSnapshot(ctx)
	if err != nil {
		return nil, err
	}
	// Runtime configuration and enrollment remain properties of the destination.
	config.Tables["settings"] = nil
	payload := portablePayload{Config: *config, Servers: servers, Certificates: make(map[int64]certstore.Pair)}
	for _, row := range config.Tables["certificates"] {
		if row["provider"] != "custom" {
			continue
		}
		if s.store == nil {
			return nil, fmt.Errorf("certificate backup storage is not configured")
		}
		id, err := backupID(row["id"])
		if err != nil {
			return nil, err
		}
		certPath, _ := row["cert_path"].(string)
		keyPath, _ := row["key_path"].(string)
		pair, err := s.store.Load(certPath, keyPath)
		if err != nil {
			return nil, fmt.Errorf("read certificate %d for backup: %w", id, err)
		}
		payload.Certificates[id] = pair
		row["cert_path"] = nil
		row["key_path"] = nil
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}
	if len(raw) > 32<<20 {
		return nil, fmt.Errorf("portable backup exceeds size limit")
	}
	ciphertext, err := security.Encrypt(key, string(raw))
	if err != nil {
		return nil, err
	}
	return json.Marshal(portableEnvelope{Version: portableVersion, Salt: base64.StdEncoding.EncodeToString(salt), Ciphertext: ciphertext})
}
func (s *BackupService) RestorePortable(ctx context.Context, data []byte, opts PortableRestoreOptions) (int, error) {
	if len(data) > maxPortableSize {
		return 0, fmt.Errorf("backup exceeds size limit")
	}
	var envelope portableEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil || envelope.Version != portableVersion {
		return 0, fmt.Errorf("invalid or unsupported encrypted backup")
	}
	salt, err := base64.StdEncoding.DecodeString(envelope.Salt)
	if err != nil {
		return 0, fmt.Errorf("invalid backup salt")
	}
	key, err := portableKey(opts.Passphrase, salt)
	if err != nil {
		return 0, err
	}
	plaintext, err := security.Decrypt(key, envelope.Ciphertext)
	if err != nil {
		return 0, fmt.Errorf("backup authentication failed; check the passphrase and file")
	}
	if len(plaintext) > 32<<20 {
		return 0, fmt.Errorf("backup exceeds size limit")
	}
	var payload portablePayload
	if err := json.Unmarshal([]byte(plaintext), &payload); err != nil {
		return 0, fmt.Errorf("invalid backup payload")
	}
	if err := validateBackup(&payload.Config); err != nil {
		return 0, err
	}
	if len(payload.Config.Tables["settings"]) != 0 {
		return 0, fmt.Errorf("portable backup cannot replace runtime settings")
	}
	known := map[int64]portableServer{}
	mapping := map[int64]int64{1: 1}
	used := map[int64]bool{1: true}
	for _, srv := range payload.Servers {
		if srv.ID < 1 || known[srv.ID].ID != 0 || srv.Local != (srv.ID == 1) || len(srv.Name) > 255 || len(srv.Hostname) > 255 {
			return 0, fmt.Errorf("invalid backup server descriptor")
		}
		known[srv.ID] = srv
	}
	if known[1].ID != 1 {
		return 0, fmt.Errorf("backup lacks local server identity")
	}
	for source, target := range opts.ServerMapping {
		if source <= 1 || known[source].ID == 0 || target <= 1 || used[target] {
			return 0, fmt.Errorf("invalid or duplicate server mapping")
		}
		var exists int
		if err := s.db.QueryRowContext(ctx, `SELECT 1 FROM servers WHERE id=? AND is_local=0`, target).Scan(&exists); err != nil {
			return 0, fmt.Errorf("mapped destination server does not exist")
		}
		mapping[source] = target
		used[target] = true
	}
	for id := range known {
		if id > 1 && mapping[id] == 0 && !opts.CreateUnenrolled {
			return 0, fmt.Errorf("remote server %d requires an explicit mapping or create_unenrolled", id)
		}
	}
	certRows := map[int64]map[string]any{}
	for _, row := range payload.Config.Tables["certificates"] {
		id, err := backupID(row["id"])
		if err != nil || certRows[id] != nil {
			return 0, fmt.Errorf("invalid or duplicate certificate ID")
		}
		certRows[id] = row
		if row["provider"] == "custom" {
			pair, ok := payload.Certificates[id]
			if !ok {
				return 0, fmt.Errorf("missing certificate material")
			}
			var domains []string
			domainJSON, _ := row["domain_names"].(string)
			if err := json.Unmarshal([]byte(domainJSON), &domains); err != nil || len(domains) == 0 {
				return 0, fmt.Errorf("invalid certificate domains")
			}
			if _, err := certstore.Validate(pair, domains); err != nil {
				return 0, fmt.Errorf("invalid certificate %d: %w", id, err)
			}
			if s.store == nil {
				return 0, fmt.Errorf("certificate recovery storage is not configured")
			}
		} else if _, ok := payload.Certificates[id]; ok {
			return 0, fmt.Errorf("unexpected certificate material")
		}
	}
	for id := range payload.Certificates {
		if certRows[id] == nil {
			return 0, fmt.Errorf("unreferenced certificate material")
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	for id, srv := range known {
		if id > 1 && mapping[id] == 0 {
			result, err := tx.ExecContext(ctx, `INSERT INTO servers(name,hostname,deployment_type,status,agent_token_hash,ssh_credentials_encrypted,ssh_host_key) VALUES(?,?,'manual','pending','','','')`, srv.Name, srv.Hostname)
			if err != nil {
				return 0, err
			}
			target, err := result.LastInsertId()
			if err != nil {
				return 0, err
			}
			mapping[id] = target
		}
	}
	for _, table := range []string{"proxy_hosts", "redirection_hosts", "stream_hosts", "dead_hosts"} {
		for _, row := range payload.Config.Tables[table] {
			id := int64(1)
			if value, ok := row["server_id"]; ok {
				id, err = backupID(value)
				if err != nil {
					return 0, err
				}
			}
			if mapping[id] == 0 {
				return 0, fmt.Errorf("host refers to unknown backup server")
			}
			row["server_id"] = mapping[id]
		}
	}
	// Resolve all SQL constraints before staging new immutable certificate sources.
	settings, err := tx.QueryContext(ctx, `SELECT key,value FROM settings`)
	if err != nil {
		return 0, err
	}
	for settings.Next() {
		var k, v string
		if err := settings.Scan(&k, &v); err != nil {
			settings.Close()
			return 0, err
		}
		payload.Config.Tables["settings"] = append(payload.Config.Tables["settings"], map[string]any{"key": k, "value": v})
	}
	if err := settings.Err(); err != nil {
		settings.Close()
		return 0, err
	}
	settings.Close()
	restored, err := replaceBackupTables(ctx, tx, &payload.Config)
	if err != nil {
		return 0, err
	}
	for id, pair := range payload.Certificates {
		certPath, keyPath, err := s.store.Save(id, pair)
		if err != nil {
			return 0, fmt.Errorf("stage recovered certificate: %w", err)
		}
		if _, err := tx.ExecContext(ctx, `UPDATE certificates SET cert_path=?,key_path=? WHERE id=?`, certPath, keyPath, id); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return restored, s.applyRestored(ctx)
}
func backupID(value any) (int64, error) {
	var text string
	switch v := value.(type) {
	case int64:
		if v > 0 {
			return v, nil
		}
	case float64:
		if v > 0 && v < 1<<53 && v == float64(int64(v)) {
			return int64(v), nil
		}
	case json.Number:
		text = v.String()
	}
	if text != "" {
		id, err := strconv.ParseInt(text, 10, 64)
		if err == nil && id > 0 {
			return id, nil
		}
	}
	return 0, fmt.Errorf("invalid backup identifier")
}
