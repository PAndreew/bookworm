package auth

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/base32"
	"encoding/binary"
	"fmt"
	"net/url"
	"strings"
	"time"
)

// TOTP per RFC 6238: 6 digits, 30-second step, HMAC-SHA1. SHA1 is what every
// authenticator app actually implements; the construction is not weakened by
// SHA1's collision problems.
const (
	totpDigits = 6
	totpPeriod = 30 * time.Second
	// totpSkew allows one step either side, covering clock drift between the
	// phone and the server. Wider than this materially helps an attacker.
	totpSkew = 1
)

// NewTOTPSecret returns 20 random bytes, the RFC 4226 recommended length.
func NewTOTPSecret() ([]byte, error) {
	secret := make([]byte, 20)
	if _, err := rand.Read(secret); err != nil {
		return nil, fmt.Errorf("auth: generating totp secret: %w", err)
	}
	return secret, nil
}

// TOTPProvisioningURI builds the otpauth:// URI that becomes the enrolment QR code.
func TOTPProvisioningURI(secret []byte, accountEmail string) string {
	const issuer = "Bookworm"
	v := url.Values{}
	v.Set("secret", base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(secret))
	v.Set("issuer", issuer)
	v.Set("algorithm", "SHA1")
	v.Set("digits", fmt.Sprint(totpDigits))
	v.Set("period", fmt.Sprint(int(totpPeriod.Seconds())))

	label := url.PathEscape(issuer + ":" + accountEmail)
	return "otpauth://totp/" + label + "?" + v.Encode()
}

// VerifyTOTP reports whether code is valid for secret at time now, allowing one
// step of clock skew in each direction.
func VerifyTOTP(secret []byte, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if len(code) != totpDigits {
		return false
	}

	counter := now.Unix() / int64(totpPeriod.Seconds())
	for offset := -totpSkew; offset <= totpSkew; offset++ {
		want := hotp(secret, uint64(counter+int64(offset)))
		// Constant time even though the code is short-lived: it still leaks.
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true
		}
	}
	return false
}

// hotp implements RFC 4226 truncation.
func hotp(secret []byte, counter uint64) string {
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)

	mac := hmac.New(sha1.New, secret)
	mac.Write(buf[:])
	sum := mac.Sum(nil)

	offset := sum[len(sum)-1] & 0x0f
	value := binary.BigEndian.Uint32(sum[offset:offset+4]) & 0x7fffffff

	mod := uint32(1)
	for i := 0; i < totpDigits; i++ {
		mod *= 10
	}
	return fmt.Sprintf("%0*d", totpDigits, value%mod)
}

// --------------------------------------------------------------- recovery codes

// NewRecoveryCodes returns n human-transcribable codes plus their SHA-256 hashes.
// The plaintext is shown to the user exactly once; only the hashes are stored.
func NewRecoveryCodes(n int) (plain []string, hashes [][]byte, err error) {
	// Crockford-style alphabet: no I, L, O, U, so codes survive being read aloud
	// or copied off a screenshot.
	const alphabet = "0123456789ABCDEFGHJKMNPQRSTVWXYZ"
	const codeLen = 10

	for i := 0; i < n; i++ {
		buf := make([]byte, codeLen)
		if _, err := rand.Read(buf); err != nil {
			return nil, nil, fmt.Errorf("auth: generating recovery code: %w", err)
		}
		var sb strings.Builder
		for j, b := range buf {
			if j == codeLen/2 {
				sb.WriteByte('-')
			}
			// Modulo bias is negligible: 256 % 32 == 0, so this is uniform.
			sb.WriteByte(alphabet[int(b)%len(alphabet)])
		}
		code := sb.String()
		sum := sha256.Sum256([]byte(code))

		plain = append(plain, code)
		hashes = append(hashes, sum[:])
	}
	return plain, hashes, nil
}

// HashRecoveryCode hashes a user-submitted code for comparison against storage.
func HashRecoveryCode(code string) []byte {
	normalised := strings.ToUpper(strings.TrimSpace(code))
	sum := sha256.Sum256([]byte(normalised))
	return sum[:]
}
