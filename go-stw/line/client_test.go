package line

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"testing"
)

func TestValidateSignature(t *testing.T) {
	secret := "test_secret_key"
	body := []byte(`{"events":[]}`)

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	sig := base64.StdEncoding.EncodeToString(mac.Sum(nil))

	if !ValidateSignature(secret, body, sig) {
		t.Fatalf("expected valid signature to pass")
	}

	if ValidateSignature(secret, body, "invalid_signature") {
		t.Fatalf("expected invalid signature to fail")
	}

	// Empty secret passes
	if !ValidateSignature("", body, "any") {
		t.Fatalf("expected empty secret to pass")
	}
}
