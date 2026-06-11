package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/models"
)

// AccessListService manages IP restriction rules stored in SQLite.
type AccessListService struct {
	db    *sql.DB
	audit *AuditService
}

// NewAccessListService creates an AccessListService backed by db.
func NewAccessListService(db *sql.DB, audit *AuditService) *AccessListService {
	return &AccessListService{db: db, audit: audit}
}

// ── Input types ───────────────────────────────────────────────────────────────

// AccessListClientInput is a single IP/CIDR rule submitted by the API.
type AccessListClientInput struct {
	Address   string `json:"address"`
	Directive string `json:"directive"`
	SortOrder int    `json:"sort_order"`
}

// CreateAccessListInput carries all fields accepted when creating or updating
// an AccessList.
type CreateAccessListInput struct {
	Name    string                  `json:"name"`
	Clients []AccessListClientInput `json:"clients"`
}

// ── CRUD ──────────────────────────────────────────────────────────────────────

func (s *AccessListService) List(ctx context.Context) ([]models.AccessList, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, name, created_at, updated_at
		FROM access_lists ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var lists []models.AccessList
	for rows.Next() {
		al, err := scanAccessListRow(rows)
		if err != nil {
			return nil, err
		}
		lists = append(lists, *al)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	for i := range lists {
		if err := s.hydrateAccessList(ctx, s.db, &lists[i]); err != nil {
			return nil, err
		}
	}
	return lists, nil
}

func (s *AccessListService) GetByID(ctx context.Context, id int64) (*models.AccessList, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, name, created_at, updated_at
		FROM access_lists WHERE id = ?`, id)
	al, err := scanAccessListRow(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if err := s.hydrateAccessList(ctx, s.db, al); err != nil {
		return nil, err
	}
	return al, nil
}

func (s *AccessListService) Create(ctx context.Context, callerID *int64, input CreateAccessListInput, ip string) (*models.AccessList, error) {
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	result, err := tx.ExecContext(ctx, `INSERT INTO access_lists (name) VALUES (?)`, input.Name)
	if err != nil {
		return nil, fmt.Errorf("insert access list: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}

	if err := insertClients(ctx, tx, id, input.Clients); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	al, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.audit.Log(ctx, callerID, "create", "access_list", &id, map[string]any{"name": input.Name}, ip)
	return al, nil
}

func (s *AccessListService) Update(ctx context.Context, callerID *int64, id int64, input CreateAccessListInput, ip string) (*models.AccessList, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil || existing == nil {
		return nil, fmt.Errorf("access list not found")
	}
	if input.Name == "" {
		return nil, fmt.Errorf("name is required")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `
		UPDATE access_lists SET name=?, updated_at=datetime('now') WHERE id=?`,
		input.Name, id,
	); err != nil {
		return nil, fmt.Errorf("update access list: %w", err)
	}

	if _, err := tx.ExecContext(ctx, "DELETE FROM access_list_clients WHERE access_list_id = ?", id); err != nil {
		return nil, fmt.Errorf("delete clients: %w", err)
	}
	if err := insertClients(ctx, tx, id, input.Clients); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit: %w", err)
	}

	al, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	s.audit.Log(ctx, callerID, "update", "access_list", &id, map[string]any{"name": input.Name}, ip)
	return al, nil
}

func (s *AccessListService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	existing, err := s.GetByID(ctx, id)
	if err != nil || existing == nil {
		return fmt.Errorf("access list not found")
	}
	if _, err := s.db.ExecContext(ctx, "DELETE FROM access_lists WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete access list: %w", err)
	}
	s.audit.Log(ctx, callerID, "delete", "access_list", &id, nil, ip)
	return nil
}

// ── helpers ───────────────────────────────────────────────────────────────────

type dbQuerier interface {
	ExecContext(ctx context.Context, query string, args ...any) (sql.Result, error)
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, query string, args ...any) *sql.Row
}

func insertClients(ctx context.Context, q dbQuerier, listID int64, clients []AccessListClientInput) error {
	for _, c := range clients {
		if c.Address == "" {
			continue
		}
		if c.Directive != "allow" && c.Directive != "deny" {
			c.Directive = "allow"
		}
		if _, err := q.ExecContext(ctx, `
			INSERT INTO access_list_clients (access_list_id, address, directive, sort_order)
			VALUES (?,?,?,?)`,
			listID, c.Address, c.Directive, c.SortOrder,
		); err != nil {
			return fmt.Errorf("insert client: %w", err)
		}
	}
	return nil
}

func (s *AccessListService) hydrateAccessList(ctx context.Context, q dbQuerier, al *models.AccessList) error {
	rows, err := q.QueryContext(ctx, `
		SELECT id, access_list_id, address, directive, sort_order
		FROM access_list_clients WHERE access_list_id = ? ORDER BY sort_order`, al.ID)
	if err != nil {
		return err
	}
	defer rows.Close()
	for rows.Next() {
		var c models.AccessListClient
		if err := rows.Scan(&c.ID, &c.AccessListID, &c.Address, &c.Directive, &c.SortOrder); err != nil {
			return err
		}
		al.Clients = append(al.Clients, c)
	}
	return rows.Err()
}

func scanAccessListRow(row scanner) (*models.AccessList, error) {
	var al models.AccessList
	err := row.Scan(&al.ID, &al.Name, &al.CreatedAt, &al.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &al, nil
}
