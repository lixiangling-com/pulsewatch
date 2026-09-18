package auth

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"fmt"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v4"
	"github.com/google/uuid"
	"github.com/lixiangling-com/pulsewatch/server/internal/platform/httpx"
)

const (
	AccessTokenLifetime  = 15 * time.Minute
	RefreshTokenLifetime = 7 * 24 * time.Hour
	UserIDContextKey     = "auth.user_id"
)

var ErrInvalidToken = errors.New("invalid token")

type TokenManager struct {
	secret []byte
	issuer string
	now    func() time.Time
}

func NewTokenManager(secret, issuer string) (*TokenManager, error) {
	if len(secret) < 32 || issuer == "" {
		return nil, errors.New("invalid token configuration")
	}
	return &TokenManager{secret: []byte(secret), issuer: issuer, now: time.Now}, nil
}

func (m *TokenManager) NewAccessToken(userID uuid.UUID) (string, error) {
	now := m.now().UTC()
	claims := jwt.RegisteredClaims{
		Subject:   userID.String(),
		Issuer:    m.issuer,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(AccessTokenLifetime)),
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString(m.secret)
}

func (m *TokenManager) ParseAccessToken(raw string) (uuid.UUID, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(raw, claims, func(token *jwt.Token) (any, error) {
		if token.Method != jwt.SigningMethodHS256 {
			return nil, ErrInvalidToken
		}
		return m.secret, nil
	}, jwt.WithValidMethods([]string{jwt.SigningMethodHS256.Alg()}))
	if err != nil || !token.Valid || !claims.VerifyIssuer(m.issuer, true) || !claims.VerifyExpiresAt(m.now(), true) {
		return uuid.Nil, ErrInvalidToken
	}
	userID, err := uuid.Parse(claims.Subject)
	if err != nil {
		return uuid.Nil, ErrInvalidToken
	}
	return userID, nil
}

func NewRefreshToken() (string, [sha256.Size]byte, error) {
	var raw [32]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return "", [sha256.Size]byte{}, fmt.Errorf("generate refresh token: %w", err)
	}
	encoded := base64.RawURLEncoding.EncodeToString(raw[:])
	return encoded, sha256.Sum256([]byte(encoded)), nil
}

func HashRefreshToken(raw string) [sha256.Size]byte {
	return sha256.Sum256([]byte(raw))
}

func (m *TokenManager) RequireUser() gin.HandlerFunc {
	return func(c *gin.Context) {
		raw, ok := bearerToken(c.GetHeader("Authorization"))
		if !ok {
			httpx.WriteError(c, 401, "UNAUTHORIZED", "认证已失效，请重新登录", nil)
			return
		}
		userID, err := m.ParseAccessToken(raw)
		if err != nil {
			httpx.WriteError(c, 401, "UNAUTHORIZED", "认证已失效，请重新登录", nil)
			return
		}
		c.Set(UserIDContextKey, userID)
		c.Next()
	}
}

func UserIDFromContext(c *gin.Context) (uuid.UUID, bool) {
	userID, ok := c.Get(UserIDContextKey)
	value, valid := userID.(uuid.UUID)
	return value, ok && valid
}

func bearerToken(header string) (string, bool) {
	const prefix = "Bearer "
	if len(header) <= len(prefix) || header[:len(prefix)] != prefix {
		return "", false
	}
	return header[len(prefix):], true
}
