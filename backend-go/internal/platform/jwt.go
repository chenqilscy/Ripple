package platform

import (
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// ErrInvalidToken Token 解析或校验失败。
var ErrInvalidToken = errors.New("invalid token")

// Claims 是 Ripple 的 JWT 载荷（强类型）。
type Claims struct {
	UserID string `json:"sub"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}

// JWTSigner 签发与解析 Token。
type JWTSigner struct {
	secret    []byte
	expiresIn time.Duration
}

// NewJWTSigner 创建签名器。
func NewJWTSigner(secret string, expiresIn time.Duration) *JWTSigner {
	return &JWTSigner{secret: []byte(secret), expiresIn: expiresIn}
}

// Sign 签发一个新 Token。
func (s *JWTSigner) Sign(userID, email string) (string, error) {
	now := time.Now()
	c := Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(s.expiresIn)),
			Issuer:    "ripple",
		},
	}
	t := jwt.NewWithClaims(jwt.SigningMethodHS256, c)
	str, err := t.SignedString(s.secret)
	if err != nil {
		return "", fmt.Errorf("jwt sign: %w", err)
	}
	return str, nil
}

// Parse 校验并解析 Token。
func (s *JWTSigner) Parse(tokenStr string) (*Claims, error) {
	c := &Claims{}
	tok, err := jwt.ParseWithClaims(tokenStr, c, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secret, nil
	})
	if err != nil || !tok.Valid {
		return nil, ErrInvalidToken
	}
	return c, nil
}
