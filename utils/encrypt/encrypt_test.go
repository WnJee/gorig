package encrypt

import "testing"

func TestEncryptUsesAuthenticatedCiphertext(t *testing.T) {
	key := GenerateKey()
	ciphertext, err := Encrypt("secret", key)
	if err != nil {
		t.Fatal(err)
	}
	plaintext, err := Decrypt(ciphertext, key)
	if err != nil || plaintext != "secret" {
		t.Fatalf("round trip failed: plaintext=%q err=%v", plaintext, err)
	}
	prefixLen := len("gcm:")
	tampered := ciphertext[:prefixLen] + "A" + ciphertext[prefixLen+1:]
	if _, err := Decrypt(tampered, key); err == nil {
		t.Fatal("tampered authenticated ciphertext must be rejected")
	}
}

func TestDecryptRejectsMalformedCiphertext(t *testing.T) {
	key := GenerateKey()
	if key == "" {
		t.Fatal("failed to generate test key")
	}
	for _, ciphertext := range []string{"", "AA==", "AAAAAAAAAAAAAAAAAAAAAA=="} {
		if _, err := Decrypt(ciphertext, key); err == nil {
			t.Fatalf("expected malformed ciphertext %q to be rejected", ciphertext)
		}
	}
}
