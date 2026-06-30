// Package crypto provides password hashing (bcrypt), symmetric encryption
// (AES-256-GCM), and HMAC signing primitives for the TikTok-clone platform.
package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"crypto/sha512"
	"crypto/subtle"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"hash"
	"io"

	"golang.org/x/crypto/bcrypt"
)

// ---- Password hashing (bcrypt) ----------------------------------------------

const (
	// DefaultBcryptCost is a reasonable default that keeps bcrypt at ~100 ms on
	// modern hardware. Increase to 13-14 for higher security at the cost of
	// latency.
	DefaultBcryptCost = 12
	// MinBcryptCost is the lowest accepted cost value.
	MinBcryptCost = bcrypt.MinCost
	// MaxBcryptCost is the highest accepted cost value.
	MaxBcryptCost = bcrypt.MaxCost
)

// HashPassword hashes password with bcrypt at DefaultBcryptCost.
// Returns the encoded hash string suitable for storage.
func HashPassword(password string) (string, error) {
	return HashPasswordWithCost(password, DefaultBcryptCost)
}

// HashPasswordWithCost hashes password with the specified bcrypt cost.
func HashPasswordWithCost(password string, cost int) (string, error) {
	if cost < MinBcryptCost || cost > MaxBcryptCost {
		return "", fmt.Errorf("crypto: bcrypt cost %d out of range [%d, %d]",
			cost, MinBcryptCost, MaxBcryptCost)
	}
	if len(password) == 0 {
		return "", errors.New("crypto: password must not be empty")
	}
	// bcrypt silently truncates input at 72 bytes; protect against that.
	if len(password) > 72 {
		return "", errors.New("crypto: password exceeds 72-byte bcrypt limit")
	}
	b, err := bcrypt.GenerateFromPassword([]byte(password), cost)
	if err != nil {
		return "", fmt.Errorf("crypto: hashing password: %w", err)
	}
	return string(b), nil
}

// CheckPassword compares a plain-text password against a bcrypt hash.
// Returns nil if they match, or an error (including ErrPasswordMismatch).
func CheckPassword(password, hash string) error {
	if err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)); err != nil {
		if errors.Is(err, bcrypt.ErrMismatchedHashAndPassword) {
			return ErrPasswordMismatch
		}
		return fmt.Errorf("crypto: comparing password: %w", err)
	}
	return nil
}

// NeedsRehash reports whether the stored hash was produced with a cost lower
// than the current DefaultBcryptCost and should be rehashed on next login.
func NeedsRehash(hash string) bool {
	cost, err := bcrypt.Cost([]byte(hash))
	if err != nil {
		return true
	}
	return cost < DefaultBcryptCost
}

// ErrPasswordMismatch is returned by CheckPassword when the password does not
// match the hash.
var ErrPasswordMismatch = errors.New("crypto: password does not match")

// ---- AES-256-GCM encryption -------------------------------------------------

const (
	// AES256KeySize is the required key length in bytes.
	AES256KeySize = 32
	// GCMNonceSize is the GCM nonce length (96 bits).
	GCMNonceSize = 12
	// GCMTagSize is the GCM authentication tag length.
	GCMTagSize = 16
)

// GenerateKey generates a cryptographically random AES-256 key.
func GenerateKey() ([]byte, error) {
	key := make([]byte, AES256KeySize)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, fmt.Errorf("crypto: generating key: %w", err)
	}
	return key, nil
}

// GenerateKeyHex returns a hex-encoded AES-256 key.
func GenerateKeyHex() (string, error) {
	key, err := GenerateKey()
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(key), nil
}

// Encrypt encrypts plaintext using AES-256-GCM with the provided key.
// The returned ciphertext is in the format: nonce || ciphertext || tag,
// base64url-encoded.
func Encrypt(plaintext, key []byte) (string, error) {
	if len(key) != AES256KeySize {
		return "", fmt.Errorf("crypto: key must be %d bytes, got %d", AES256KeySize, len(key))
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", fmt.Errorf("crypto: creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("crypto: creating GCM: %w", err)
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("crypto: generating nonce: %w", err)
	}

	// Seal appends ciphertext + tag after nonce.
	sealed := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(sealed), nil
}

// EncryptString is a convenience wrapper that operates on strings.
func EncryptString(plaintext string, key []byte) (string, error) {
	return Encrypt([]byte(plaintext), key)
}

// EncryptWithHexKey decodes a hex-encoded key and encrypts plaintext.
func EncryptWithHexKey(plaintext []byte, hexKey string) (string, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return "", fmt.Errorf("crypto: decoding hex key: %w", err)
	}
	return Encrypt(plaintext, key)
}

// Decrypt decrypts a base64url-encoded AES-256-GCM ciphertext.
func Decrypt(ciphertext64 string, key []byte) ([]byte, error) {
	if len(key) != AES256KeySize {
		return nil, fmt.Errorf("crypto: key must be %d bytes, got %d", AES256KeySize, len(key))
	}

	data, err := base64.RawURLEncoding.DecodeString(ciphertext64)
	if err != nil {
		return nil, fmt.Errorf("crypto: decoding ciphertext: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, fmt.Errorf("crypto: creating AES cipher: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("crypto: creating GCM: %w", err)
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize+GCMTagSize {
		return nil, errors.New("crypto: ciphertext too short")
	}

	nonce, cipherData := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, cipherData, nil)
	if err != nil {
		return nil, fmt.Errorf("crypto: decrypting: %w", err)
	}
	return plaintext, nil
}

// DecryptString decrypts to a string.
func DecryptString(ciphertext64 string, key []byte) (string, error) {
	b, err := Decrypt(ciphertext64, key)
	if err != nil {
		return "", err
	}
	return string(b), nil
}

// DecryptWithHexKey decodes a hex-encoded key and decrypts ciphertext.
func DecryptWithHexKey(ciphertext64, hexKey string) ([]byte, error) {
	key, err := hex.DecodeString(hexKey)
	if err != nil {
		return nil, fmt.Errorf("crypto: decoding hex key: %w", err)
	}
	return Decrypt(ciphertext64, key)
}

// ---- HMAC signing -----------------------------------------------------------

// HMACAlgorithm selects the underlying hash function.
type HMACAlgorithm string

const (
	HMACSHA256 HMACAlgorithm = "sha256"
	HMACSHA512 HMACAlgorithm = "sha512"
)

// Sign computes an HMAC signature over data with key using the specified
// algorithm. Returns a hex-encoded signature.
func Sign(data, key []byte, alg HMACAlgorithm) (string, error) {
	h, err := newHMAC(key, alg)
	if err != nil {
		return "", err
	}
	h.Write(data)
	return hex.EncodeToString(h.Sum(nil)), nil
}

// SignString is a convenience wrapper that accepts string data and key.
func SignString(data, key string, alg HMACAlgorithm) (string, error) {
	return Sign([]byte(data), []byte(key), alg)
}

// Verify verifies that the provided hex-encoded signature matches the data
// and key.  Uses constant-time comparison to prevent timing attacks.
func Verify(data, key []byte, alg HMACAlgorithm, signature string) (bool, error) {
	expected, err := Sign(data, key, alg)
	if err != nil {
		return false, err
	}
	return subtle.ConstantTimeCompare([]byte(expected), []byte(signature)) == 1, nil
}

// VerifyString is a convenience wrapper for string data and key.
func VerifyString(data, key, signature string, alg HMACAlgorithm) (bool, error) {
	return Verify([]byte(data), []byte(key), alg, signature)
}

func newHMAC(key []byte, alg HMACAlgorithm) (hash.Hash, error) {
	switch alg {
	case HMACSHA256:
		return hmac.New(sha256.New, key), nil
	case HMACSHA512:
		return hmac.New(sha512.New, key), nil
	default:
		return nil, fmt.Errorf("crypto: unsupported HMAC algorithm %q", alg)
	}
}

// ---- Utility ----------------------------------------------------------------

// RandomBytes returns n cryptographically random bytes.
func RandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	if _, err := io.ReadFull(rand.Reader, b); err != nil {
		return nil, fmt.Errorf("crypto: generating random bytes: %w", err)
	}
	return b, nil
}

// RandomHex returns a hex-encoded string of n random bytes (2n hex chars).
func RandomHex(n int) (string, error) {
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// RandomBase64 returns a base64url-encoded string of n random bytes.
func RandomBase64(n int) (string, error) {
	b, err := RandomBytes(n)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

// SHA256Hex returns the hex-encoded SHA-256 digest of data.
func SHA256Hex(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}

// SHA512Hex returns the hex-encoded SHA-512 digest of data.
func SHA512Hex(data []byte) string {
	h := sha512.Sum512(data)
	return hex.EncodeToString(h[:])
}
