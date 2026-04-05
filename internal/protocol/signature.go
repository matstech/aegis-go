package protocol

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
)

// Sign computes the Aegis request signature using HMAC-SHA512 and standard base64 encoding.
func Sign(secret, canonical string) string {
	signer := hmac.New(sha512.New, []byte(secret))
	signer.Write([]byte(canonical))
	return base64.StdEncoding.EncodeToString(signer.Sum(nil))
}
