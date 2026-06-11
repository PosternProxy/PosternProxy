package auth

import "testing"

func TestHashPassword_Produces_ValidHash(t *testing.T) {
	hash, err := HashPassword("mysecretpw")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	if hash == "" {
		t.Fatal("hash is empty")
	}
	if hash == "mysecretpw" {
		t.Fatal("hash is plaintext")
	}
}

func TestCheckPassword_CorrectPassword(t *testing.T) {
	hash, _ := HashPassword("correct-horse-battery")
	if !CheckPassword(hash, "correct-horse-battery") {
		t.Error("CheckPassword returned false for correct password")
	}
}

func TestCheckPassword_WrongPassword(t *testing.T) {
	hash, _ := HashPassword("correct-horse-battery")
	if CheckPassword(hash, "wrong-password") {
		t.Error("CheckPassword returned true for wrong password")
	}
}

func TestCheckPassword_EmptyPassword(t *testing.T) {
	hash, _ := HashPassword("notempty")
	if CheckPassword(hash, "") {
		t.Error("CheckPassword returned true for empty password")
	}
}

func TestHashPassword_DifferentHashes(t *testing.T) {
	h1, _ := HashPassword("same-password")
	h2, _ := HashPassword("same-password")
	if h1 == h2 {
		t.Error("same password should produce different hashes (salt)")
	}
}
