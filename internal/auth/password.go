package auth

import "github.com/posternproxy/posternproxy/internal/security"

// HashPassword returns a bcrypt hash of password suitable for storage.
func HashPassword(password string) (string, error) {
	return security.BcryptHash(password)
}

// CheckPassword reports whether password matches the stored bcrypt hash.
func CheckPassword(hash, password string) bool {
	return security.BcryptCompare(hash, password) == nil
}
