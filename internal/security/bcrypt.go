package security

import "golang.org/x/crypto/bcrypt"

const bcryptCost = 12

// BcryptHash hashes a plaintext password with cost 12.
func BcryptHash(plaintext string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(plaintext), bcryptCost)
	return string(b), err
}

// BcryptCompare returns nil if plaintext matches the bcrypt hash.
func BcryptCompare(hash, plaintext string) error {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plaintext))
}
