package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"task_crud_api/internal/model"
)

const issuer = "task_crud_api"

// AccessClaims is what the signed access token carries. Role and Name ride
// along so authorization and display need no database hit; RegisteredClaims
// supplies the standard sub/exp/iat/iss fields.
//
// Nothing secret goes in here: a JWT is signed, not encrypted, so anyone can
// base64-decode and read it. The signature only proves it was not tampered with.
type AccessClaims struct {
	Role model.Role `json:"role"`
	Name string     `json:"name"`
	jwt.RegisteredClaims
}

// TokenService issues and verifies tokens. It holds the signing secret and the
// two lifetimes, so nothing else in the app needs to know them.
type TokenService struct {
	secret     []byte
	accessTTL  time.Duration
	refreshTTL time.Duration
}

func NewTokenService(secret string, accessTTL, refreshTTL time.Duration) *TokenService {
	return &TokenService{
		secret:     []byte(secret),
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

func (s *TokenService) RefreshTTL() time.Duration { return s.refreshTTL }

// IssueAccess signs a short-lived access token for a user.
func (s *TokenService) IssueAccess(u model.User, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(s.accessTTL)
	claims := AccessClaims{
		Role: u.Role,
		Name: u.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   strconv.Itoa(u.ID),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString(s.secret)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

// ParseAccess verifies the signature and expiry, then returns the user the
// token stands for, built from its claims alone. It returns model.ErrUnauthorized
// for anything wrong, so the caller never has to tell the failure modes apart.
func (s *TokenService) ParseAccess(raw string) (model.User, error) {
	var claims AccessClaims

	// The keyfunc pins the algorithm: without this check an attacker could send
	// a token with alg=none, or an RSA public key as an HMAC secret, and forge
	// claims. Accept only the method we signed with.
	_, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return s.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}), jwt.WithIssuer(issuer))
	if err != nil {
		return model.User{}, model.ErrUnauthorized
	}

	id, err := strconv.Atoi(claims.Subject)
	if err != nil {
		return model.User{}, model.ErrUnauthorized
	}
	return model.User{ID: id, Role: claims.Role, Name: claims.Name}, nil
}

// NewRefreshToken returns a random opaque token and the SHA-256 hash to store.
// The plain token goes to the client once; only the hash is persisted, so a
// database leak does not hand over working tokens. SHA-256 (not bcrypt) is
// right here: the token is already 256 bits of entropy, so it needs no slow
// hash to resist guessing, and lookups must be fast.
func (s *TokenService) NewRefreshToken() (plain string, hash string, expiresAt time.Time, err error) {
	b := make([]byte, 32)
	if _, err = rand.Read(b); err != nil {
		return "", "", time.Time{}, err
	}
	plain = hex.EncodeToString(b)
	hash = HashRefreshToken(plain)
	expiresAt = time.Now().Add(s.refreshTTL)
	return plain, hash, expiresAt, nil
}

// HashRefreshToken is the one place that hashes a refresh token, so issuing and
// looking up cannot drift apart.
func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
