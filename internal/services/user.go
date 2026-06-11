package services

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/posternproxy/posternproxy/internal/auth"
	"github.com/posternproxy/posternproxy/internal/models"
)

// UserService manages application user accounts in SQLite.
type UserService struct {
	db    *sql.DB
	audit *AuditService
}

// NewUserService creates a UserService backed by db.
func NewUserService(db *sql.DB, audit *AuditService) *UserService {
	return &UserService{db: db, audit: audit}
}

// List returns all users ordered by ID.
func (s *UserService) List(ctx context.Context) ([]models.User, error) {
	rows, err := s.db.QueryContext(ctx, "SELECT id, email, name, password_hash, role, is_active, created_at, updated_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var users []models.User
	for rows.Next() {
		var u models.User
		if err := rows.Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		users = append(users, u)
	}
	return users, rows.Err()
}

// GetByID returns the user with the given ID, or nil if not found.
func (s *UserService) GetByID(ctx context.Context, id int64) (*models.User, error) {
	var u models.User
	err := s.db.QueryRowContext(ctx,
		"SELECT id, email, name, password_hash, role, is_active, created_at, updated_at FROM users WHERE id = ?", id,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetByEmail returns the user with the given email address, or nil if not found.
func (s *UserService) GetByEmail(ctx context.Context, email string) (*models.User, error) {
	var u models.User
	err := s.db.QueryRowContext(ctx,
		"SELECT id, email, name, password_hash, role, is_active, created_at, updated_at FROM users WHERE email = ?", email,
	).Scan(&u.ID, &u.Email, &u.Name, &u.PasswordHash, &u.Role, &u.IsActive, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// Create hashes the provided password and inserts a new user record.
func (s *UserService) Create(ctx context.Context, callerID *int64, input models.CreateUserInput, ip string) (*models.User, error) {
	if len(input.Password) < 8 {
		return nil, fmt.Errorf("password must be at least 8 characters")
	}
	if input.Role != "admin" && input.Role != "viewer" {
		input.Role = "viewer"
	}

	hash, err := auth.HashPassword(input.Password)
	if err != nil {
		return nil, fmt.Errorf("hash password: %w", err)
	}

	result, err := s.db.ExecContext(ctx,
		"INSERT INTO users (email, name, password_hash, role) VALUES (?, ?, ?, ?)",
		input.Email, input.Name, hash, input.Role,
	)
	if err != nil {
		return nil, fmt.Errorf("insert user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	user, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, fmt.Errorf("fetch created user: %w", err)
	}

	_ = s.audit.Log(ctx, callerID, "create", "user", &id, map[string]string{"email": input.Email, "role": input.Role}, ip)
	return user, nil
}

func (s *UserService) Update(ctx context.Context, callerID *int64, id int64, input models.UpdateUserInput, ip string) (*models.User, error) {
	existing, err := s.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing == nil {
		return nil, fmt.Errorf("user not found")
	}

	if input.Email != nil {
		existing.Email = *input.Email
	}
	if input.Name != nil {
		existing.Name = *input.Name
	}
	if input.Role != nil {
		existing.Role = *input.Role
	}
	if input.IsActive != nil {
		existing.IsActive = *input.IsActive
	}

	_, err = s.db.ExecContext(ctx,
		"UPDATE users SET email = ?, name = ?, role = ?, is_active = ?, updated_at = datetime('now') WHERE id = ?",
		existing.Email, existing.Name, existing.Role, existing.IsActive, id,
	)
	if err != nil {
		return nil, fmt.Errorf("update user: %w", err)
	}

	_ = s.audit.Log(ctx, callerID, "update", "user", &id, input, ip)
	return s.GetByID(ctx, id)
}

func (s *UserService) ChangePassword(ctx context.Context, callerID *int64, id int64, password, ip string) error {
	if len(password) < 8 {
		return fmt.Errorf("password must be at least 8 characters")
	}

	hash, err := auth.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	_, err = s.db.ExecContext(ctx, "UPDATE users SET password_hash = ?, updated_at = datetime('now') WHERE id = ?", hash, id)
	if err != nil {
		return fmt.Errorf("update password: %w", err)
	}

	_ = s.audit.Log(ctx, callerID, "change_password", "user", &id, nil, ip)
	return nil
}

func (s *UserService) Delete(ctx context.Context, callerID *int64, id int64, ip string) error {
	result, err := s.db.ExecContext(ctx, "DELETE FROM users WHERE id = ?", id)
	if err != nil {
		return fmt.Errorf("delete user: %w", err)
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return fmt.Errorf("user not found")
	}

	_ = s.audit.Log(ctx, callerID, "delete", "user", &id, nil, ip)
	return nil
}

func (s *UserService) Count(ctx context.Context) (int, error) {
	var count int
	err := s.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM users").Scan(&count)
	return count, err
}
