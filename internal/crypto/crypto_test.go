package crypto

import (
	"bytes"
	"errors"
	"testing"
)

var testKey = []byte("0123456789abcdef0123456789abcdef")

func TestEncryptDecryptRoundTrip(t *testing.T) {
	plaintext := []byte("hello, world! русский текст")
	ciphertext, err := Encrypt(testKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	if bytes.Equal(ciphertext, plaintext) {
		t.Fatal("ciphertext equals plaintext")
	}
	decrypted, err := Decrypt(testKey, ciphertext)
	if err != nil {
		t.Fatalf("Decrypt: %v", err)
	}
	if !bytes.Equal(decrypted, plaintext) {
		t.Fatalf("got %q, want %q", decrypted, plaintext)
	}
}

func TestEncryptProducesDifferentOutput(t *testing.T) {
	plaintext := []byte("same input")
	c1, err := Encrypt(testKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	c2, err := Encrypt(testKey, plaintext)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}
	if bytes.Equal(c1, c2) {
		t.Fatal("two encryptions produced identical ciphertext (nonce reuse?)")
	}
}

func TestDecryptWrongKey(t *testing.T) {
	wrongKey := []byte("ffffffffffffffffffffffffffffffff")
	ciphertext, err := Encrypt(testKey, []byte("secret"))
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	_, err = Decrypt(wrongKey, ciphertext)
	if err == nil {
		t.Fatal("expected error with wrong key, got nil")
	}
}

func TestDecryptTooShort(t *testing.T) {
	_, err := Decrypt(testKey, []byte{0x01, 0x02})
	if !errors.Is(err, ErrCiphertextTooShort) {
		t.Fatalf("got %v, want ErrCiphertextTooShort", err)
	}
}

func TestDecryptTamperedCiphertext(t *testing.T) {
	ciphertext, _ := Encrypt(testKey, []byte("important data"))
	ciphertext[len(ciphertext)-1] ^= 0xFF // портим последний байт (tag)
	if _, err := Decrypt(testKey, ciphertext); err == nil {
		t.Fatal("expected authentication error, got nil")
	}
}