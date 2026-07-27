package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"strings"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

// testKey mints a throwaway RSA key so tests need no key files on disk.
func testKey(t *testing.T) *rsa.PrivateKey {
	t.Helper()
	k, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("generate test key: %v", err)
	}
	return k
}

func TestAccessTokenRoundTrip(t *testing.T) {
	svc := NewTokenService(testKey(t), 15*time.Minute, 7*24*time.Hour)
	user := model.User{ID: uuid.New(), Role: model.RoleAdmin, Name: "Alice"}

	token, _, err := svc.IssueAccess(user, time.Now())
	if err != nil {
		t.Fatalf("IssueAccess returned an error: %v", err)
	}

	got, err := svc.ParseAccess(token)
	if err != nil {
		t.Fatalf("ParseAccess rejected a token we just issued: %v", err)
	}

	if got.ID != user.ID {
		t.Errorf("id: got %v, want %v", got.ID, user.ID)
	}
	if got.Role != user.Role {
		t.Errorf("role: got %q, want %q", got.Role, user.Role)
	}
	if got.Name != user.Name {
		t.Errorf("name: got %q, want %q", got.Name, user.Name)
	}
}

func TestParseAccessRejectsBadTokens(t *testing.T) {
	svc := NewTokenService(testKey(t), 15*time.Minute, time.Hour)
	imposter := NewTokenService(testKey(t), 15*time.Minute, time.Hour) // a different keypair
	user := model.User{ID: uuid.New(), Role: model.RoleMember, Name: "Bob"}

	valid, _, _ := svc.IssueAccess(user, time.Now())
	expired, _, _ := svc.IssueAccess(user, time.Now().Add(-time.Hour))
	forged, _, _ := imposter.IssueAccess(user, time.Now()) // right shape, wrong key

	tampered := tamperSignature(valid)

	// An HS256 token whose "secret" is the RSA public key — the classic
	// algorithm-confusion forgery. Alg-pinning to RS256 must reject it.
	algConfusion := hs256TokenSignedWith(t, svc.verifyKey, user)

	cases := []struct {
		name  string
		token string
	}{
		{"empty string", ""},
		{"not a jwt at all", "hello.world.nope"},
		{"tampered signature", tampered},
		{"expired token", expired},
		{"signed with the wrong key", forged},
		{"algorithm confusion (HS256)", algConfusion},
	}

	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			if _, err := svc.ParseAccess(tt.token); err == nil {
				t.Fatalf("ParseAccess accepted a token it should have rejected")
			}
		})
	}
}

// tamperSignature flips the FIRST character of the signature segment, which
// encodes high-order bits and so always changes the decoded signature bytes.
// (Flipping the LAST char can be a no-op: it may only touch ignored padding
// bits of a 256-byte RS256 signature, leaving the signature valid.)
func tamperSignature(token string) string {
	i := strings.LastIndex(token, ".")
	sig := token[i+1:]
	first := "A"
	if sig[0] == 'A' {
		first = "B"
	}
	return token[:i+1] + first + sig[1:]
}

// hs256TokenSignedWith forges an HS256 token using the RSA public key's PEM as
// the HMAC secret — the attack that a naive verifier (no alg check) falls for.
func hs256TokenSignedWith(t *testing.T, pub *rsa.PublicKey, u model.User) string {
	t.Helper()
	claims := AccessClaims{
		Role: u.Role,
		Name: u.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(time.Hour)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	// The bytes an attacker would use: the (public) modulus. Value doesn't
	// matter — the point is the token claims alg=HS256, which we must refuse.
	signed, err := tok.SignedString(pub.N.Bytes())
	if err != nil {
		t.Fatalf("forge HS256 token: %v", err)
	}
	return signed
}
