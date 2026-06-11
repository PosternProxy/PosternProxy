package models

// User is an application user who can authenticate and manage resources.
// PasswordHash is never serialised to JSON.
type User struct {
	ID           int64  `json:"id"`
	Email        string `json:"email"`
	Name         string `json:"name"`
	PasswordHash string `json:"-"`
	Role         string `json:"role"`
	IsActive     bool   `json:"is_active"`
	CreatedAt    string `json:"created_at"`
	UpdatedAt    string `json:"updated_at"`
}

// CreateUserInput carries the fields required to register a new user.
type CreateUserInput struct {
	Email    string `json:"email"`
	Name     string `json:"name"`
	Password string `json:"password"`
	Role     string `json:"role"`
}

// UpdateUserInput carries the optional fields that may be changed on an existing
// user. Nil fields are left unchanged.
type UpdateUserInput struct {
	Email    *string `json:"email,omitempty"`
	Name     *string `json:"name,omitempty"`
	Role     *string `json:"role,omitempty"`
	IsActive *bool   `json:"is_active,omitempty"`
}

// ChangePasswordInput carries the new plain-text password for a password-change
// request. It is hashed before storage and never persisted in this form.
type ChangePasswordInput struct {
	Password string `json:"password"`
}
