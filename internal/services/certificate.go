package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/posternproxy/posternproxy/internal/models"
)

// CertificateService manages TLS certificates in the database and on disk.
type CertificateService struct {
	db      *sql.DB
	certDir string
	audit   *AuditService
}

// NewCertificateService creates a CertificateService that stores uploaded
// certificate files under certDir.
func NewCertificateService(db *sql.DB, certDir string, audit *AuditService) *CertificateService {
	return &CertificateService{db: db, certDir: certDir, audit: audit}
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

// List returns all certificates ordered by ID.
func (s *CertificateService) List(ctx context.Context) ([]*models.Certificate, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, provider, domain_names, cert_path, key_path,
		       expires_at, status, error_message, created_at, updated_at
		FROM certificates ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var certs []*models.Certificate
	for rows.Next() {
		c, err := scanCertRow(rows)
		if err != nil {
			return nil, err
		}
		certs = append(certs, c)
	}
	return certs, rows.Err()
}

// GetByID returns the certificate with the given ID, or nil if not found.
func (s *CertificateService) GetByID(ctx context.Context, id int64) (*models.Certificate, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, provider, domain_names, cert_path, key_path,
		       expires_at, status, error_message, created_at, updated_at
		FROM certificates WHERE id = ?`, id)
	c, err := scanCertRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return c, err
}

// UploadCustom saves PEM certificate and key data to disk under certDir and
// inserts a "custom" provider record in the database.
func (s *CertificateService) UploadCustom(ctx context.Context, callerID *int64, name string, domainNames json.RawMessage, certReader, keyReader io.Reader, ip string) (*models.Certificate, error) {
	if err := validateCertDomains(domainNames); err != nil {
		return nil, err
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	// Insert a placeholder row first to get an ID for the directory name.
	result, err := s.db.ExecContext(ctx, `
		INSERT INTO certificates (name, provider, domain_names, status) VALUES (?, 'custom', ?, 'pending')`,
		name, string(domainNames),
	)
	if err != nil {
		return nil, fmt.Errorf("insert certificate: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	// Write files under certDir/id/
	dir := filepath.Join(s.certDir, fmt.Sprintf("%d", id))
	if err := os.MkdirAll(dir, 0700); err != nil {
		return nil, fmt.Errorf("create cert dir: %w", err)
	}

	certPath := filepath.Join(dir, "cert.pem")
	keyPath := filepath.Join(dir, "key.pem")

	if err := writeFile(certPath, certReader); err != nil {
		return nil, fmt.Errorf("save cert file: %w", err)
	}
	if err := writeFile(keyPath, keyReader); err != nil {
		return nil, fmt.Errorf("save key file: %w", err)
	}

	// Update row with actual paths and mark active.
	if _, err := s.db.ExecContext(ctx, `
		UPDATE certificates SET cert_path=?, key_path=?, status='active', updated_at=datetime('now')
		WHERE id=?`,
		certPath, keyPath, id,
	); err != nil {
		return nil, fmt.Errorf("update cert paths: %w", err)
	}

	cert, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	_ = s.audit.Log(ctx, callerID, "create", "certificate", &id, map[string]any{"name": name, "provider": "custom"}, ip)
	return cert, nil
}

func (s *CertificateService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	cert, err := s.GetByID(ctx, id)
	if err != nil || cert == nil {
		return fmt.Errorf("certificate not found")
	}

	if _, err := s.db.ExecContext(ctx, "DELETE FROM certificates WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete certificate: %w", err)
	}

	// Remove cert files from disk if present
	if cert.CertPath != nil {
		dir := filepath.Dir(*cert.CertPath)
		_ = os.RemoveAll(dir)
	}

	_ = s.audit.Log(ctx, callerID, "delete", "certificate", &id, nil, ip)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

func writeFile(path string, r io.Reader) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.Copy(f, r)
	return err
}

func validateCertDomains(domainNames json.RawMessage) error {
	var domains []string
	if err := json.Unmarshal(domainNames, &domains); err != nil || len(domains) == 0 {
		return fmt.Errorf("at least one domain name is required")
	}
	return nil
}

type certScanner interface {
	Scan(dest ...any) error
}

func scanCertRow(row certScanner) (*models.Certificate, error) {
	var c models.Certificate
	var domainNames string
	err := row.Scan(
		&c.ID, &c.Name, &c.Provider, &domainNames,
		&c.CertPath, &c.KeyPath,
		&c.ExpiresAt, &c.Status, &c.ErrorMessage,
		&c.CreatedAt, &c.UpdatedAt,
	)
	if err != nil {
		return nil, err
	}
	c.DomainNames = json.RawMessage(domainNames)
	return &c, nil
}
