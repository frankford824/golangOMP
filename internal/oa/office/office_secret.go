package office

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"strings"
)

func officeCipher() (cipher.AEAD, error) {
	raw, err := base64.StdEncoding.DecodeString(os.Getenv("OA_OFFICE_ENCRYPTION_KEY"))
	if err != nil || len(raw) != 32 {
		return nil, errors.New("OA office encryption key unavailable")
	}
	block, err := aes.NewCipher(raw)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}

func sealOfficeValue(value string) (string, error) { return sealBoundOfficeValue(value, "draft") }
func openOfficeValue(value string) (string, error) { return openBoundOfficeValue(value, "draft") }

func sealBoundOfficeValue(value, context string) (string, error) {
	a, err := officeCipher()
	if err != nil {
		return "", err
	}
	nonce := make([]byte, a.NonceSize())
	if _, err = rand.Read(nonce); err != nil {
		return "", err
	}
	ciphertext := a.Seal(nonce, nonce, []byte(value), []byte(context))
	return "v1." + base64.RawStdEncoding.EncodeToString(ciphertext), nil
}
func openBoundOfficeValue(value, context string) (string, error) {
	a, err := officeCipher()
	if err != nil {
		return "", err
	}
	encoded, ok := strings.CutPrefix(value, "v1.")
	if !ok {
		return "", fmt.Errorf("unsupported office ciphertext")
	}
	data, err := base64.RawStdEncoding.DecodeString(encoded)
	if err != nil || len(data) < a.NonceSize()+a.Overhead() {
		return "", errors.New("invalid office ciphertext")
	}
	plain, err := a.Open(nil, data[:a.NonceSize()], data[a.NonceSize():], []byte(context))
	return string(plain), err
}
