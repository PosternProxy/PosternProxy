package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"github.com/posternproxy/posternproxy/internal/app"
	"github.com/posternproxy/posternproxy/internal/config"
	"github.com/posternproxy/posternproxy/internal/database"
	"github.com/posternproxy/posternproxy/internal/models"
	"github.com/posternproxy/posternproxy/internal/services"
)

// runResetPassword implements `posternproxy reset-password [--email <addr>]`.
// It opens the database directly (no HTTP server needed), finds the target
// user, generates a new random password, and prints it.
func runResetPassword(args []string) {
	fs := flag.NewFlagSet("reset-password", flag.ExitOnError)
	email := fs.String("email", "", "email address of the account to reset (default: first admin)")
	fs.Usage = func() {
		fmt.Fprintln(os.Stderr, "Usage: posternproxy reset-password [--email <address>]")
		fs.PrintDefaults()
	}
	_ = fs.Parse(args)

	ctx := context.Background()
	cfg := config.Load()

	db, err := database.Open(cfg.DBPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: open database: %v\n", err)
		os.Exit(1)
	}
	defer db.Close()

	auditSvc := services.NewAuditService(db)
	userSvc := services.NewUserService(db, auditSvc)

	user, err := findAdminUser(ctx, userSvc, *email)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	newPassword := app.RandomPassword()
	if err := userSvc.ChangePassword(ctx, nil, user.ID, newPassword, "cli"); err != nil {
		fmt.Fprintf(os.Stderr, "error: reset password: %v\n", err)
		os.Exit(1)
	}

	printResetBanner(user.Email, newPassword)
}

// findAdminUser resolves the target of `reset-password`. If email is empty,
// the first admin in the database is used.
func findAdminUser(ctx context.Context, userSvc *services.UserService, email string) (*models.User, error) {
	if email != "" {
		user, err := userSvc.GetByEmail(ctx, email)
		if err != nil {
			return nil, fmt.Errorf("lookup user: %w", err)
		}
		if user == nil {
			return nil, fmt.Errorf("no user found with email %q", email)
		}
		return user, nil
	}

	users, err := userSvc.List(ctx)
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}
	for i := range users {
		if users[i].Role == "admin" {
			return &users[i], nil
		}
	}
	return nil, fmt.Errorf("no admin user found in database")
}
