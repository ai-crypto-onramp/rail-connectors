package webhooks

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

// Verify reports whether sig is a valid HMAC-SHA256 hex digest of body under
// secret. Returns ErrBadSignature on mismatch.
var ErrBadSignature = errors.New("webhook: bad signature")

// Verify checks the X-Webhook-Signature header value against the body.
func Verify(body []byte, sig string, secret string) error {
	want, err := compute(body, secret)
	if err != nil {
		return err
	}
	if !hmac.Equal([]byte(sig), []byte(want)) {
		return ErrBadSignature
	}
	return nil
}

// Compute returns the expected HMAC-SHA256 hex digest for body under secret.
func Compute(body []byte, secret string) string {
	out, _ := compute(body, secret)
	return out
}

func compute(body []byte, secret string) (string, error) {
	mac := hmac.New(sha256.New, []byte(secret))
	if _, err := mac.Write(body); err != nil {
		return "", err
	}
	return hex.EncodeToString(mac.Sum(nil)), nil
}
