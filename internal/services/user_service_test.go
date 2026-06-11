package services

import (
	"context"
	"testing"

	"github.com/posternproxy/posternproxy/internal/models"
)

func newUserSvc(t *testing.T) *UserService {
	t.Helper()
	db := testDB(t)
	audit := NewAuditService(db)
	return NewUserService(db, audit)
}

func TestUserService_CreateAndGetByID(t *testing.T) {
	svc := newUserSvc(t)

	input := models.CreateUserInput{
		Email:    "alice@example.com",
		Name:     "Alice",
		Password: "password1",
		Role:     "admin",
	}
	user, err := svc.Create(context.Background(), nil, input, "127.0.0.1")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.ID == 0 {
		t.Fatal("expected non-zero ID")
	}
	if user.Email != "alice@example.com" {
		t.Errorf("email = %q, want %q", user.Email, "alice@example.com")
	}
	if user.Role != "admin" {
		t.Errorf("role = %q, want admin", user.Role)
	}
	if user.PasswordHash == "password1" {
		t.Error("password stored as plaintext")
	}

	got, err := svc.GetByID(context.Background(), user.ID)
	if err != nil {
		t.Fatalf("GetByID: %v", err)
	}
	if got == nil {
		t.Fatal("GetByID returned nil")
	}
	if got.Email != user.Email {
		t.Errorf("email mismatch: got %q", got.Email)
	}
}

func TestUserService_GetByID_NotFound(t *testing.T) {
	svc := newUserSvc(t)
	got, err := svc.GetByID(context.Background(), 9999)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}
}

func TestUserService_GetByEmail(t *testing.T) {
	svc := newUserSvc(t)

	input := models.CreateUserInput{Email: "bob@example.com", Name: "Bob", Password: "s3cr3t99", Role: "viewer"}
	created, _ := svc.Create(context.Background(), nil, input, "")

	got, err := svc.GetByEmail(context.Background(), "bob@example.com")
	if err != nil {
		t.Fatalf("GetByEmail: %v", err)
	}
	if got == nil || got.ID != created.ID {
		t.Errorf("GetByEmail returned wrong user")
	}

	missing, err := svc.GetByEmail(context.Background(), "nobody@example.com")
	if err != nil {
		t.Fatalf("GetByEmail missing: %v", err)
	}
	if missing != nil {
		t.Error("expected nil for unknown email")
	}
}

func TestUserService_List(t *testing.T) {
	svc := newUserSvc(t)

	for _, name := range []string{"U1", "U2", "U3"} {
		_, err := svc.Create(context.Background(), nil, models.CreateUserInput{
			Email:    name + "@example.com",
			Name:     name,
			Password: "abcdefgh",
		}, "")
		if err != nil {
			t.Fatalf("Create %s: %v", name, err)
		}
	}

	users, err := svc.List(context.Background())
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(users) < 3 {
		t.Errorf("List returned %d users, want >= 3", len(users))
	}
}

func TestUserService_Update(t *testing.T) {
	svc := newUserSvc(t)

	user, _ := svc.Create(context.Background(), nil, models.CreateUserInput{Email: "charlie@example.com", Name: "Charlie", Password: "abcdefgh"}, "")

	newEmail := "charlie2@example.com"
	newName := "Charlie Two"
	newRole := "admin"
	active := false
	updated, err := svc.Update(context.Background(), nil, user.ID, models.UpdateUserInput{
		Email:    &newEmail,
		Name:     &newName,
		Role:     &newRole,
		IsActive: &active,
	}, "")
	if err != nil {
		t.Fatalf("Update: %v", err)
	}
	if updated.Email != newEmail {
		t.Errorf("email = %q, want %q", updated.Email, newEmail)
	}
	if updated.Name != newName {
		t.Errorf("name = %q, want %q", updated.Name, newName)
	}
	if updated.Role != "admin" {
		t.Errorf("role = %q, want admin", updated.Role)
	}
	if updated.IsActive != false {
		t.Error("is_active should be false")
	}
}

func TestUserService_ChangePassword(t *testing.T) {
	svc := newUserSvc(t)

	user, _ := svc.Create(context.Background(), nil, models.CreateUserInput{Email: "dave@example.com", Name: "Dave", Password: "oldpass1"}, "")
	oldHash := user.PasswordHash

	if err := svc.ChangePassword(context.Background(), nil, user.ID, "newpass99", ""); err != nil {
		t.Fatalf("ChangePassword: %v", err)
	}

	updated, _ := svc.GetByID(context.Background(), user.ID)
	if updated.PasswordHash == oldHash {
		t.Error("password hash unchanged after ChangePassword")
	}
}

func TestUserService_ChangePassword_TooShort(t *testing.T) {
	svc := newUserSvc(t)
	user, _ := svc.Create(context.Background(), nil, models.CreateUserInput{Email: "e@e.com", Name: "E", Password: "12345678"}, "")
	if err := svc.ChangePassword(context.Background(), nil, user.ID, "short", ""); err == nil {
		t.Error("expected error for short password")
	}
}

func TestUserService_Delete(t *testing.T) {
	svc := newUserSvc(t)

	user, _ := svc.Create(context.Background(), nil, models.CreateUserInput{Email: "frank@example.com", Name: "Frank", Password: "abcdefgh"}, "")

	if err := svc.Delete(context.Background(), nil, user.ID, ""); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	got, _ := svc.GetByID(context.Background(), user.ID)
	if got != nil {
		t.Error("user still exists after Delete")
	}
}

func TestUserService_Delete_NotFound(t *testing.T) {
	svc := newUserSvc(t)
	if err := svc.Delete(context.Background(), nil, 9999, ""); err == nil {
		t.Error("expected error deleting non-existent user")
	}
}

func TestUserService_Create_DefaultRole(t *testing.T) {
	svc := newUserSvc(t)
	user, err := svc.Create(context.Background(), nil, models.CreateUserInput{Email: "g@g.com", Name: "G", Password: "12345678", Role: "superuser"}, "")
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if user.Role != "viewer" {
		t.Errorf("unknown role should default to viewer, got %q", user.Role)
	}
}

func TestUserService_Count(t *testing.T) {
	svc := newUserSvc(t)
	count0, _ := svc.Count(context.Background())
	svc.Create(context.Background(), nil, models.CreateUserInput{Email: "h@h.com", Name: "H", Password: "12345678"}, "")
	count1, _ := svc.Count(context.Background())
	if count1 != count0+1 {
		t.Errorf("Count = %d, want %d", count1, count0+1)
	}
}
