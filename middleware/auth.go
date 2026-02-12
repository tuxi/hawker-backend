package middleware

import (
	"hawker-backend/utils"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware(jwtKey string) gin.HandlerFunc {
	return func(c *gin.Context) {
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" || !strings.HasPrefix(authHeader, "Bearer ") {
			c.JSON(401, gin.H{"error": "未登录"})
			c.Abort()
			return
		}

		tokenString := authHeader[7:] // 去掉 "Bearer "
		claims := &utils.Claims{}

		token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
			return jwtKey, nil
		})

		if err != nil || !token.Valid {
			c.JSON(401, gin.H{"error": "无效的Token"})
			c.Abort()
			return
		}

		// 将 OwnerID 存入上下文，方便后续 Handler 直接使用
		c.Set("current_owner_id", claims.OwnerID)
		c.Next()
	}
}
