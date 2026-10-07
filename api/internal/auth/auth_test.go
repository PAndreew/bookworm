package auth

import (
	"strings"
	"testing"
	"time"
)

func TestHashPasswordVerifies(t *testing.T) {
	const pw = "correct horse battery staple"

	hash, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := VerifyPassword(pw, hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if !ok {
		t.Error("the correct password failed to verify")
	}
}

func TestVerifyPasswordRejectsWrongPassword(t *testing.T) {
	hash, err := HashPassword("correct horse battery staple")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := VerifyPassword("Correct horse battery staple", hash) // one capital differs
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("a wrong password verified successfully")
	}
}

func TestHashPasswordSaltsEachHash(t *testing.T) {
	const pw = "the same password twice"

	a, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}
	b, err := HashPassword(pw)
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	if a == b {
		t.Error("identical passwords produced identical hashes; the salt is not random")
	}
}

func TestHashPasswordRejectsShortPassword(t *testing.T) {
	if _, err := HashPassword("short"); err != ErrPasswordTooShort {
		t.Errorf("got %v, want ErrPasswordTooShort", err)
	}
}

func TestPasswordIsNotTruncated(t *testing.T) {
	// bcrypt silently ignores everything past 72 bytes, so two different long
	// passwords collide. Argon2id must not do that.
	base := strings.Repeat("a", 72)

	hash, err := HashPassword(base + "ONE")
	if err != nil {
		t.Fatalf("HashPassword: %v", err)
	}

	ok, err := VerifyPassword(base+"TWO", hash)
	if err != nil {
		t.Fatalf("VerifyPassword: %v", err)
	}
	if ok {
		t.Error("passwords differing only after byte 72 collided; input is being truncated")
	}
}

func TestVerifyPasswordRejectsMalformedHash(t *testing.T) {
	for _, bad := range []string{"", "not-a-hash", "$argon2id$broken"} {
		if _, err := VerifyPassword("whatever", bad); err == nil {
			t.Errorf("VerifyPassword(%q) returned no error, want ErrInvalidHash", bad)
		}
	}
}

// --------------------------------------------------------------------- TOTP

func TestVerifyTOTPAcceptsCurrentCode(t *testing.T) {
	secret, err := NewTOTPSecret()
	if err != nil {
		t.Fatalf("NewTOTPSecret: %v", err)
	}

	now := time.Now()
	code := hotp(secret, uint64(now.Unix()/30))

	if !VerifyTOTP(secret, code, now) {
		t.Error("the current code failed to verify")
	}
}

func TestVerifyTOTPToleratesClockSkew(t *testing.T) {
	secret, _ := NewTOTPSecret()
	now := time.Now()

	for _, step := range []int64{-1, 1} {
		code := hotp(secret, uint64(now.Unix()/30+step))
		if !VerifyTOTP(secret, code, now) {
			t.Errorf("code from step %+d should be accepted within the skew window", step)
		}
	}
}

func TestVerifyTOTPRejectsCodeOutsideWindow(t *testing.T) {
	secret, _ := NewTOTPSecret()
	now := time.Now()

	code := hotp(secret, uint64(now.Unix()/30+5)) // well outside the window
	if VerifyTOTP(secret, code, now) {
		t.Error("a code from five steps away was accepted")
	}
}

func TestVerifyTOTPRejectsGarbage(t *testing.T) {
	secret, _ := NewTOTPSecret()
	now := time.Now()

	for _, bad := range []string{"", "12345", "1234567", "abcdef"} {
		if VerifyTOTP(secret, bad, now) {
			t.Errorf("garbage code %q was accepted", bad)
		}
	}
}

func TestTOTPAgainstRFC4226Vector(t *testing.T) {
	// RFC 4226 Appendix D: secret "12345678901234567890", counter 0 -> 755224.
	secret := []byte("12345678901234567890")
	if got := hotp(secret, 0); got != "755224" {
		t.Errorf("hotp(counter=0) = %s, want 755224 per RFC 4226", got)
	}
	if got := hotp(secret, 1); got != "287082" {
		t.Errorf("hotp(counter=1) = %s, want 287082 per RFC 4226", got)
	}
}

func TestProvisioningURIContainsIssuerAndSecret(t *testing.T) {
	secret, _ := NewTOTPSecret()
	uri := TOTPProvisioningURI(secret, "reader@example.com")

	if !strings.HasPrefix(uri, "otpauth://totp/") {
		t.Errorf("wrong scheme: %s", uri)
	}
	for _, want := range []string{"issuer=Bookworm", "secret=", "digits=6", "period=30"} {
		if !strings.Contains(uri, want) {
			t.Errorf("provisioning URI missing %q: %s", want, uri)
		}
	}
}

// ---------------------------------------------------------- recovery & sessions

func TestRecoveryCodesAreUniqueAndHashed(t *testing.T) {
	plain, hashes, err := NewRecoveryCodes(10)
	if err != nil {
		t.Fatalf("NewRecoveryCodes: %v", err)
	}
	if len(plain) != 10 || len(hashes) != 10 {
		t.Fatalf("got %d codes and %d hashes, want 10 of each", len(plain), len(hashes))
	}

	seen := map[string]bool{}
	for _, c := range plain {
		if seen[c] {
			t.Errorf("duplicate recovery code generated: %s", c)
		}
		seen[c] = true

		// Must avoid the ambiguous glyphs, or users will mistype them.
		if strings.ContainsAny(c, "ILOU") {
			t.Errorf("code %q contains an ambiguous character", c)
		}
	}
}

func TestHashRecoveryCodeIsCaseInsensitive(t *testing.T) {
	plain, hashes, _ := NewRecoveryCodes(1)

	got := HashRecoveryCode(strings.ToLower(plain[0]))
	if string(got) != string(hashes[0]) {
		t.Error("a lowercased recovery code did not match its stored hash")
	}
}

func TestSessionTokensAreUniqueAndHashConsistently(t *testing.T) {
	a, ha, err := NewSessionToken()
	if err != nil {
		t.Fatalf("NewSessionToken: %v", err)
	}
	b, _, _ := NewSessionToken()

	if a == b {
		t.Error("two session tokens were identical")
	}
	if string(HashSessionToken(a)) != string(ha) {
		t.Error("HashSessionToken disagrees with the hash returned at creation")
	}
}
