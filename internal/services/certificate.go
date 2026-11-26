package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/posternproxy/posternproxy/internal/certstore"
	"github.com/posternproxy/posternproxy/internal/models"
)

// CertificateService manages TLS certificates in the database and on disk.
type CertificateService struct {
	db            *sql.DB
	certDir       string
	audit         *AuditService
	encryptionKey *[32]byte
}

// NewCertificateService creates a CertificateService that stores uploaded
// certificate files under certDir.
func NewCertificateService(db *sql.DB, certDir string, audit *AuditService, keys ...[32]byte) *CertificateService {
	s := &CertificateService{db: db, certDir: certDir, audit: audit}
	if len(keys) == 1 {
		s.encryptionKey = &keys[0]
	}
	return s
}

func (s *CertificateService) ProtectExisting(ctx context.Context) error {
	if s.encryptionKey == nil {
		return fmt.Errorf("certificate encryption is not configured")
	}
	certs, err := s.List(ctx)
	if err != nil {
		return err
	}
	store := certstore.Store{Root: s.certDir, Key: *s.encryptionKey}
	var problems []error
	for _, cert := range certs {
		if cert.Provider != "custom" || cert.CertPath == nil || cert.KeyPath == nil {
			continue
		}
		pair, err := store.Load(*cert.CertPath, *cert.KeyPath)
		if err != nil {
			problems = append(problems, fmt.Errorf("protect stored certificate %d: %w", cert.ID, err))
			continue
		}
		certPath, keyPath, err := store.Save(cert.ID, pair)
		if err != nil {
			problems = append(problems, fmt.Errorf("stage protected certificate %d: %w", cert.ID, err))
			continue
		}
		if certPath != *cert.CertPath || keyPath != *cert.KeyPath {
			if _, err := s.db.ExecContext(ctx, `UPDATE certificates SET cert_path=?, key_path=? WHERE id=?`, certPath, keyPath, cert.ID); err != nil {
				return fmt.Errorf("record protected certificate: %w", err)
			}
		}
	}
	return errors.Join(problems...)
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

	certPEM, err := certstore.Read(certReader)
	if err != nil {
		return nil, err
	}
	keyPEM, err := certstore.Read(keyReader)
	if err != nil {
		return nil, err
	}
	var domains []string
	if err := json.Unmarshal(domainNames, &domains); err != nil {
		return nil, err
	}
	pair := certstore.Pair{Certificate: certPEM, Key: keyPEM}
	leaf, err := certstore.Validate(pair, domains)
	if err != nil {
		return nil, err
	}
	if s.encryptionKey == nil {
		return nil, fmt.Errorf("certificate encryption is not configured")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `
		INSERT INTO certificates (name, provider, domain_names, status, expires_at) VALUES (?, 'custom', ?, 'pending', ?)`,
		name, string(domainNames), leaf.NotAfter.UTC().Format(time.RFC3339))
	if err != nil {
		return nil, fmt.Errorf("insert certificate: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	store := certstore.Store{Root: s.certDir, Key: *s.encryptionKey}
	certPath, keyPath, err := store.Save(id, pair)
	if err != nil {
		return nil, fmt.Errorf("store encrypted certificate pair: %w", err)
	}

	// Update row with actual paths and mark active.
	if _, err := tx.ExecContext(ctx, `
		UPDATE certificates SET cert_path=?, key_path=?, status='active', updated_at=datetime('now')
		WHERE id=?`,
		certPath, keyPath, id,
	); err != nil {
		return nil, fmt.Errorf("update cert paths: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit certificate upload: %w", err)
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

	// Encrypted sources remain available for recovery; runtime files are retired
	// separately after successful configuration application.

	_ = s.audit.Log(ctx, callerID, "delete", "certificate", &id, nil, ip)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

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
