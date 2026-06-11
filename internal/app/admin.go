package app

import (
	"context"
	"crypto/rand"
	"fmt"
	"math/big"
	"os"

	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/rs/zerolog/log"
)

// EnsureAdminUser creates the bootstrap admin account if no users exist yet.
//
// If POSTERNPROXY_ADMIN_PASSWORD is set, that password is used and an empty
// string is returned (no banner needed). Otherwise a random 20-character
// password is generated and returned for the caller to print.
//
// Returns ("", nil) if an admin user already existed.
//
// The ctx is currently unused but accepted for future migration to
// context-aware user-service calls.
func (a *App) EnsureAdminUser(ctx context.Context) (generatedPassword string, err error) {
	count, err := a.Users.Count(ctx)
	if err != nil {
		return "", fmt.Errorf("count users: %w", err)
	}
	if count > 0 {
		return "", nil
	}

	email := os.Getenv("POSTERNPROXY_ADMIN_EMAIL")
	password := os.Getenv("POSTERNPROXY_ADMIN_PASSWORD")
	if email == "" {
		email = "admin@posternproxy.local"
	}

	generated := password == ""
	if generated {
		password = RandomPassword()
	}

	if _, err := a.Users.Create(ctx, nil, models.CreateUserInput{
		Email:    email,
		Name:     "Admin",
		Password: password,
		Role:     "admin",
	}, "system"); err != nil {
		return "", fmt.Errorf("create admin: %w", err)
	}

	if generated {
		log.Warn().Str("email", email).Msg("initial admin account created with generated password — change it immediately")
		return password, nil
	}
	log.Info().Str("email", email).Msg("created initial admin user")
	return "", nil
}

// RandomPassword generates a cryptographically random 20-character password
// using an alphanumeric charset that is safe to copy from a terminal.
//
// Exported so the reset-password CLI subcommand can reuse it.
func RandomPassword() string {
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, 20)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(charset))))
		if err != nil {
			panic("crypto/rand unavailable: " + err.Error())
		}
		b[i] = charset[n.Int64()]
	}
	return string(b)
}
