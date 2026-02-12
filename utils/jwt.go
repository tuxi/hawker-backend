package utils

import (
	"hawker-backend/conf"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

type Claims struct {
	OwnerID uuid.UUID `json:"owner_id"`
	jwt.RegisteredClaims
}

// 生成 JWT
func GenerateToken(ownerID uuid.UUID, cfg conf.AuthConfig) (string, error) {
	// 🌟 核心：动态计算过期时间
	// 将 int 转换为 time.Duration
	expirationTime := time.Now().Add(time.Duration(cfg.TokenExpireHours) * time.Hour)

	claims := &Claims{
		OwnerID: ownerID,
		RegisteredClaims: jwt.RegisteredClaims{
			// JWT 标准字段：过期时间戳
			ExpiresAt: jwt.NewNumericDate(expirationTime),
			// 建议加上：签发时间
			IssuedAt: jwt.NewNumericDate(time.Now()),
			// 建议加上：生效时间
			NotBefore: jwt.NewNumericDate(time.Now()),
		},
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString([]byte(cfg.JWTSecret))
}
