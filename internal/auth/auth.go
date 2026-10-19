// Package auth provides password hashing, JWT access tokens and random
// opaque tokens (refresh tokens and personal access tokens).
package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
)

// PATPrefix starts every personal access token, so they are recognisable in
// configuration files and distinguishable from JWTs.
const PATPrefix = "kala_pat_"

// ErrInvalidToken is returned for any token that fails verification.
var ErrInvalidToken = errors.New("invalid token")

// HashPassword hashes a password with bcrypt.
func HashPassword(pw string) (string, error) {
	h, err := bcrypt.GenerateFromPassword([]byte(pw), bcrypt.DefaultCost)
	return string(h), err
}

// CheckPassword reports whether pw matches hash.
func CheckPassword(hash, pw string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(pw)) == nil
}

// dummyHash is compared against when the user does not exist, so login timing
// does not reveal which emails are registered.
var dummyHash, _ = HashPassword("not-a-real-password")

// BurnTime spends the time a password check would take.
func BurnTime(pw string) { _ = bcrypt.CompareHashAndPassword([]byte(dummyHash), []byte(pw)) }

// Signer issues and verifies HS256 access tokens.
type Signer struct {
	key []byte
	ttl time.Duration
	now func() time.Time
}

// NewSigner returns a signer for key with the given token lifetime.
func NewSigner(key []byte, ttl time.Duration) *Signer {
	return &Signer{key: key, ttl: ttl, now: time.Now}
}

const issuer = "kala"

// Issue returns a signed token for userID and its expiry.
func (s *Signer) Issue(userID int64) (string, time.Time, error) {
	now := s.now()
	exp := now.Add(s.ttl)
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
		Issuer:    issuer,
		Subject:   strconv.FormatInt(userID, 10),
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(exp),
	})
	signed, err := tok.SignedString(s.key)
	return signed, exp, err
}

// Verify checks a token and returns its user id.
func (s *Signer) Verify(token string) (int64, error) {
	var claims jwt.RegisteredClaims
	_, err := jwt.ParseWithClaims(token, &claims, func(*jwt.Token) (any, error) { return s.key, nil },
		jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}),
		jwt.WithIssuer(issuer),
		jwt.WithExpirationRequired(),
		jwt.WithTimeFunc(s.now),
	)
	if err != nil {
		return 0, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	id, err := strconv.ParseInt(claims.Subject, 10, 64)
	if err != nil {
		return 0, ErrInvalidToken
	}
	return id, nil
}

// NewOpaque returns a random token with prefix and the hash to store.
func NewOpaque(prefix string) (token, hash string) {
	b := make([]byte, 32)
	_, _ = rand.Read(b)
	token = prefix + base64.RawURLEncoding.EncodeToString(b)
	return token, HashOpaque(token)
}

// HashOpaque hashes an opaque token for storage and lookup. The tokens carry
// 256 bits of entropy, so a fast hash is sufficient.
func HashOpaque(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

type ctxKey struct{}

// WithUser returns a context carrying the authenticated user id.
func WithUser(ctx context.Context, id int64) context.Context {
	return context.WithValue(ctx, ctxKey{}, id)
}

// UserID returns the authenticated user id from ctx (0 if none).
func UserID(ctx context.Context) int64 {
	id, _ := ctx.Value(ctxKey{}).(int64)
	return id
}
