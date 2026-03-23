package vpn

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"fmt"
	"io"
	"strings"

	"golang.org/x/crypto/curve25519"
)

// KeyPair represents a Curve25519 key pair for key exchange
type KeyPair struct {
	PrivateKey [32]byte
	PublicKey  [32]byte
}

// GenerateKeyPair creates a new Curve25519 key pair
func GenerateKeyPair() (*KeyPair, error) {
	var privateKey [32]byte
	if _, err := io.ReadFull(rand.Reader, privateKey[:]); err != nil {
		return nil, fmt.Errorf("failed to generate private key: %w", err)
	}

	// Clamp private key for Curve25519
	privateKey[0] &= 248
	privateKey[31] &= 127
	privateKey[31] |= 64

	var publicKey [32]byte
	curve25519.ScalarBaseMult(&publicKey, &privateKey)

	return &KeyPair{
		PrivateKey: privateKey,
		PublicKey:  publicKey,
	}, nil
}

// ComputeSharedSecret computes the shared secret using ECDH
func ComputeSharedSecret(privateKey, peerPublicKey [32]byte) ([32]byte, error) {
	var sharedSecret [32]byte
	out, err := curve25519.X25519(privateKey[:], peerPublicKey[:])
	if err != nil {
		return sharedSecret, fmt.Errorf("failed to compute shared secret: %w", err)
	}
	copy(sharedSecret[:], out)
	return sharedSecret, nil
}

// DeriveKey derives an AES-256 key from shared secret and additional data
func DeriveKey(sharedSecret [32]byte, info []byte) [32]byte {
	h := sha256.New()
	h.Write(sharedSecret[:])
	h.Write(info)
	var key [32]byte
	copy(key[:], h.Sum(nil))
	return key
}

// Cipher handles encryption and decryption
type Cipher struct {
	aead cipher.AEAD
}

// NewCipher creates a new AES-GCM cipher from a key
func NewCipher(key [32]byte) (*Cipher, error) {
	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, fmt.Errorf("failed to create cipher: %w", err)
	}

	aead, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("failed to create GCM: %w", err)
	}

	return &Cipher{aead: aead}, nil
}

// Encrypt encrypts plaintext with AES-GCM
func (c *Cipher) Encrypt(plaintext []byte) ([]byte, error) {
	nonce := make([]byte, c.aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("failed to generate nonce: %w", err)
	}

	ciphertext := c.aead.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt decrypts ciphertext with AES-GCM
func (c *Cipher) Decrypt(ciphertext []byte) ([]byte, error) {
	if len(ciphertext) < c.aead.NonceSize() {
		return nil, fmt.Errorf("ciphertext too short")
	}

	nonce := ciphertext[:c.aead.NonceSize()]
	ciphertext = ciphertext[c.aead.NonceSize():]

	plaintext, err := c.aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, fmt.Errorf("decryption failed: %w", err)
	}

	return plaintext, nil
}

// GenerateSecretCode creates a random 20-character base32 secret code
func GenerateSecretCode() string {
	bytes := make([]byte, 12)
	rand.Read(bytes)
	code := base32.StdEncoding.EncodeToString(bytes)
	code = strings.ToUpper(code[:20])
	return fmt.Sprintf("%s-%s-%s-%s-%s",
		code[0:4], code[4:8], code[8:12], code[12:16], code[16:20])
}

// NormalizeSecretCode removes dashes and converts to uppercase
func NormalizeSecretCode(code string) string {
	return strings.ToUpper(strings.ReplaceAll(code, "-", ""))
}

// ValidateSecretCode checks if two codes match (ignoring formatting)
func ValidateSecretCode(code1, code2 string) bool {
	return NormalizeSecretCode(code1) == NormalizeSecretCode(code2)
}

// HashSecretCode creates a hash of the secret code for verification
func HashSecretCode(code string) [32]byte {
	normalized := NormalizeSecretCode(code)
	return sha256.Sum256([]byte(normalized))
}
