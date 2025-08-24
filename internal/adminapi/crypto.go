package adminapi

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
)

// encryptJSON optionally encrypts v with GCM if encrypterKey is set; else JSON-encodes.
func (a *App) encryptJSON(v any) ([]byte, error) {
	if len(a.encrypterKey) == 0 {
		return json.Marshal(v)
	}
	plain, err := json.Marshal(v)
	if err != nil {
		return nil, err
	}
	h := sha256.Sum256(a.encrypterKey)
	block, err := aes.NewCipher(h[:])
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}
	ct := gcm.Seal(nil, nonce, plain, nil)
	out := make([]byte, 1+len(nonce)+len(ct))
	out[0] = 0x01
	copy(out[1:1+len(nonce)], nonce)
	copy(out[1+len(nonce):], ct)
	return out, nil
}
