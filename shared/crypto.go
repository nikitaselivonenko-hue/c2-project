// Файл содержит функции симметричного шифрования AES-256-GCM,
// используемые для защиты данных при обмене между компонентами системы.
package shared

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
	"io"
)

// Encrypt шифрует данные алгоритмом AES-256-GCM.
// Параметры:
//   - key — ключ шифрования длиной 32 байта (AES-256);
//   - plaintext — открытые данные для шифрования.
// Возвращает зашифрованный срез в формате: nonce (12 байт) + ciphertext + tag.
// Случайный nonce генерируется через crypto/rand для каждого вызова,
// что исключает повторное использование (nonce reuse) для одного ключа.
func Encrypt(key, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}
	// Seal дописывает зашифрованные данные и тег аутентификации после nonce
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return ciphertext, nil
}

// Decrypt расшифровывает данные, полученные от функции Encrypt.
// Параметры:
//   - key — тот же ключ, что использовался при шифровании;
//   - data — зашифрованные данные в формате nonce + ciphertext + tag.
// Возвращает расшифрованные данные или ошибку, если данные повреждены,
// имеют неверный формат или не прошли проверку аутентификации GCM.
func Decrypt(key, data []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(data) < gcm.NonceSize() {
		return nil, errors.New("ciphertext too short")
	}
	nonce := data[:gcm.NonceSize()]
	ciphertext := data[gcm.NonceSize():]
	return gcm.Open(nil, nonce, ciphertext, nil)
}
