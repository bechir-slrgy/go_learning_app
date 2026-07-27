package auth

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"

	"task_crud_api/internal/model"
)

const issuer = "task_crud_api"

type AccessClaims struct {
	Role model.Role `json:"role"`
	Name string     `json:"name"`
	jwt.RegisteredClaims
}

// TokenService signs access tokens with an RSA private key (RS256) and verifies
// them with the matching public key. Asymmetric signing means a verifier needs
// only the public key, never the secret that mints tokens.
type TokenService struct {
	signKey    *rsa.PrivateKey
	verifyKey  *rsa.PublicKey
	accessTTL  time.Duration
	refreshTTL time.Duration
}

// NewTokenService takes the RSA private key that signs access tokens; the public
// key used to verify them is derived from it, so there is only one key to manage.
func NewTokenService(signKey *rsa.PrivateKey, accessTTL, refreshTTL time.Duration) *TokenService {
	return &TokenService{
		signKey:    signKey,
		verifyKey:  &signKey.PublicKey,
		accessTTL:  accessTTL,
		refreshTTL: refreshTTL,
	}
}

// ParsePrivateKeyPEM parses a PEM-encoded RSA private key (PKCS#1 or PKCS#8),
// the format produced by `openssl genrsa` or `ssh-keygen -m PEM`.
func ParsePrivateKeyPEM(pemBytes []byte) (*rsa.PrivateKey, error) {
	key, err := jwt.ParseRSAPrivateKeyFromPEM(pemBytes)
	if err != nil {
		return nil, fmt.Errorf("parse RSA private key: %w", err)
	}
	return key, nil
}

// GenerateDevKey mints an in-memory RSA keypair for local development when no
// key file is configured. Tokens signed with it do not survive a restart.
func GenerateDevKey() (*rsa.PrivateKey, error) {
	return rsa.GenerateKey(rand.Reader, 2048)
}

func (s *TokenService) RefreshTTL() time.Duration { return s.refreshTTL }

func (s *TokenService) IssueAccess(u model.User, now time.Time) (string, time.Time, error) {
	expiresAt := now.Add(s.accessTTL)
	claims := AccessClaims{
		Role: u.Role,
		Name: u.Name,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID.String(),
			Issuer:    issuer,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(expiresAt),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodRS256, claims)
	signed, err := token.SignedString(s.signKey)
	if err != nil {
		return "", time.Time{}, fmt.Errorf("sign access token: %w", err)
	}
	return signed, expiresAt, nil
}

func (s *TokenService) ParseAccess(raw string) (model.User, error) {
	var claims AccessClaims

	_, err := jwt.ParseWithClaims(raw, &claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodRSA); !ok {
			return nil, fmt.Errorf("unexpected signing method %v", t.Header["alg"])
		}
		return s.verifyKey, nil
	}, jwt.WithValidMethods([]string{"RS256"}), jwt.WithIssuer(issuer))
	if err != nil {
		return model.User{}, model.ErrUnauthorized
	}

	id, err := uuid.Parse(claims.Subject)
	if err != nil {
		return model.User{}, model.ErrUnauthorized
	}
	return model.User{ID: id, Role: claims.Role, Name: claims.Name}, nil
}

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

func HashRefreshToken(plain string) string {
	sum := sha256.Sum256([]byte(plain))
	return hex.EncodeToString(sum[:])
}
