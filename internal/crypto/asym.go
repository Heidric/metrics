package crypto

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"os"
)

// Envelope is the transport container for hybrid encryption.
// key   = RSA-OAEP(SHA-256) encrypted AES-256 key
// nonce = AES-GCM nonce (12 bytes), not secret
// data  = AES-GCM ciphertext+tag
type Envelope struct {
	Key   string `json:"key"`
	Nonce string `json:"nonce"`
	Data  string `json:"data"`
}

// ParseRSAPublicKeyPEM reads a PEM file containing an RSA public key.
// Accepts PKIX public key (BEGIN PUBLIC KEY) or PKCS#1 (BEGIN RSA PUBLIC KEY).
func ParseRSAPublicKeyPEM(path string) (*rsa.PublicKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read pubkey: %w", err)
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("invalid PEM for public key")
	}
	switch block.Type {
	case "PUBLIC KEY":
		k, err := x509.ParsePKIXPublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKIX public key: %w", err)
		}
		pub, ok := k.(*rsa.PublicKey)
		if !ok {
			return nil, errors.New("not an RSA public key")
		}
		return pub, nil
	case "RSA PUBLIC KEY":
		pub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 public key: %w", err)
		}
		return pub, nil
	default:
		return nil, fmt.Errorf("unsupported public key type %q", block.Type)
	}
}

// ParseRSAPrivateKeyPEM reads a PEM file containing an RSA private key.
// Accepts PKCS#1 (BEGIN RSA PRIVATE KEY) or PKCS#8 (BEGIN PRIVATE KEY).
func ParseRSAPrivateKeyPEM(path string) (*rsa.PrivateKey, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read privkey: %w", err)
	}
	block, _ := pem.Decode(b)
	if block == nil {
		return nil, errors.New("invalid PEM for private key")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		k, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS1 private key: %w", err)
		}
		return k, nil
	case "PRIVATE KEY":
		k, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("parse PKCS8 private key: %w", err)
		}
		pk, ok := k.(*rsa.PrivateKey)
		if !ok {
			return nil, errors.New("not an RSA private key")
		}
		return pk, nil
	default:
		return nil, fmt.Errorf("unsupported private key type %q", block.Type)
	}
}

// EncryptFor performs hybrid encryption of plaintext for recipient's RSA public key:
//  1. Generate random 32-byte AES key, 12-byte nonce.
//  2. AES-256-GCM encrypt plaintext.
//  3. RSA-OAEP(SHA-256) encrypt AES key.
//
// Returns JSON-friendly Envelope (base64 fields).
func EncryptFor(pub *rsa.PublicKey, plaintext []byte) (*Envelope, error) {
	if pub == nil {
		return nil, errors.New("nil public key")
	}
	aesKey := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, aesKey); err != nil {
		return nil, fmt.Errorf("rand aes key: %w", err)
	}
	nonce := make([]byte, 12)
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, fmt.Errorf("rand nonce: %w", err)
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("aes new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	label := []byte{}
	encKey, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, pub, aesKey, label)
	if err != nil {
		return nil, fmt.Errorf("rsa oaep encrypt: %w", err)
	}

	return &Envelope{
		Key:   base64.StdEncoding.EncodeToString(encKey),
		Nonce: base64.StdEncoding.EncodeToString(nonce),
		Data:  base64.StdEncoding.EncodeToString(ciphertext),
	}, nil
}

// DecryptWith reverses EncryptFor using recipient's RSA private key.
func DecryptWith(priv *rsa.PrivateKey, env *Envelope) ([]byte, error) {
	if priv == nil {
		return nil, errors.New("nil private key")
	}
	if env == nil {
		return nil, errors.New("nil envelope")
	}
	encKey, err := base64.StdEncoding.DecodeString(env.Key)
	if err != nil {
		return nil, fmt.Errorf("b64 key: %w", err)
	}
	nonce, err := base64.StdEncoding.DecodeString(env.Nonce)
	if err != nil {
		return nil, fmt.Errorf("b64 nonce: %w", err)
	}
	data, err := base64.StdEncoding.DecodeString(env.Data)
	if err != nil {
		return nil, fmt.Errorf("b64 data: %w", err)
	}

	label := []byte{}
	aesKey, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, priv, encKey, label)
	if err != nil {
		return nil, fmt.Errorf("rsa oaep decrypt: %w", err)
	}
	if len(aesKey) != 32 {
		return nil, errors.New("unexpected aes key length")
	}

	block, err := aes.NewCipher(aesKey)
	if err != nil {
		return nil, fmt.Errorf("aes new: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, fmt.Errorf("gcm: %w", err)
	}
	plaintext, err := gcm.Open(nil, nonce, data, nil)
	if err != nil {
		return nil, fmt.Errorf("gcm open: %w", err)
	}
	return bytes.Clone(plaintext), nil
}
