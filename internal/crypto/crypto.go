package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"fmt"
	"io"

	"golang.org/x/crypto/argon2"
)

// .gfe file format:
//   [4B magic "GFEX"] [1B version] [32B salt] [12B nonce] [N B ciphertext+GCM tag]

const (
	magic        = "GFEX"
	formatVersion = byte(0x01)
	saltLen      = 32
	nonceLen     = 12
	keyLen       = 32

	// Argon2id params — OWASP recommended
	argonTime    = 3
	argonMemory  = 64 * 1024 // 64 MB
	argonThreads = 4
)

// Encrypt encrypts data with AES-256-GCM, key derived from passphrase via Argon2id.
// Returns binary data in the .gfe format.
func Encrypt(data []byte, passphrase string) ([]byte, error) {
	salt := make([]byte, saltLen)
	if _, err := io.ReadFull(rand.Reader, salt); err != nil {
		return nil, fmt.Errorf("generating salt: %w", err)
	}
	nonce := make([]byte, nonceLen)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("generating nonce: %w", err)
	}

	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, data, nil)

	out := make([]byte, 0, 4+1+saltLen+nonceLen+len(ciphertext))
	out = append(out, []byte(magic)...)
	out = append(out, formatVersion)
	out = append(out, salt...)
	out = append(out, nonce...)
	out = append(out, ciphertext...)
	return out, nil
}

// Decrypt decrypts a .gfe binary blob. Returns the original plaintext.
func Decrypt(data []byte, passphrase string) ([]byte, error) {
	minLen := 4 + 1 + saltLen + nonceLen + 16 // 16 = min AES-GCM tag
	if len(data) < minLen {
		return nil, errors.New("fichier invalide : trop court")
	}
	if string(data[:4]) != magic {
		return nil, errors.New("fichier invalide : magic manquant")
	}
	if data[4] != formatVersion {
		return nil, fmt.Errorf("version non supportée : %d", data[4])
	}

	offset := 5
	salt := data[offset : offset+saltLen]
	offset += saltLen
	nonce := data[offset : offset+nonceLen]
	offset += nonceLen
	ciphertext := data[offset:]

	key := deriveKey(passphrase, salt)
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, errors.New("déchiffrement échoué : passphrase incorrecte ou fichier corrompu")
	}
	return plaintext, nil
}

// IsEncrypted reports whether data is a .gfe encrypted file.
func IsEncrypted(data []byte) bool {
	return len(data) >= 4 && string(data[:4]) == magic
}

func deriveKey(passphrase string, salt []byte) []byte {
	return argon2.IDKey([]byte(passphrase), salt, argonTime, argonMemory, argonThreads, keyLen)
}
