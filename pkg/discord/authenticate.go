package discord

import (
	crypto "crypto/ed25519"
	"encoding/hex"
	"errors"
)

const (
	SignatureHeader = "x-signature-ed25519"
	TimestampHeader = "x-signature-timestamp"
)

func Authenticate(body []byte, timestamp, signature string, publicKey crypto.PublicKey) bool {
	sig, err := hex.DecodeString(signature)
	if err != nil {
		return false
	}
	msg := append([]byte(timestamp), body...)

	return crypto.Verify(publicKey, msg, sig)
}

func DecodePublicKey(publicKey string) (crypto.PublicKey, error) {
	if publicKey == "" {
		return nil, errors.New("missing public key")
	}
	return hex.DecodeString(publicKey)
}
