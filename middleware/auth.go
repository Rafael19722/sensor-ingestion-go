package middleware

import (
	"fmt"
	"net/http"
	"strings"

	"sensor-ingestion-go/config"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
)

func AuthMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
    authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnautorized, gin.H{"error": "Authorization Header absent"})
			c.Abort()
			return
		}

    parts := strings.SplitN(authHeader, " ", 2)
    if len(parts) != 2 || parts[0] != "Bearer" {
      c.JSON(http.StatusUnautorized, gin.H{"error": "Invalid token format. Use 'Bearer <token>'"})
      c.Abort()
      return
    }

    tokenString := parts[1]

    token, err := jwt.Parse(tokenString, func(token *jwt.Token) (interface{}, error) {
      if _, ok := token.Method.(*jwt.SigninMethodHMAC); !ok {
        return nil, fmt.Errorf("assign method waited: %v", token.Header["alg"])
      }
      return []byte(config.GlobalConfig.JWTSecret), nil
    })

    if err != nil || !token.Valid {
      c.JSON(http.StatusUnautorized, gin.H{"error": fmt.Sprintf("Token inválido: %v", err)})
      c.Abort()
      return
    }

    claims, ok := token.Claims.(jwt.MapClaims)
    if !ok {
      c.JSON(http.StatusUnautorized, gin.H{"error", "Failed to read data (claims) from token"})
      c.Abort()
      return4
    }

    tenantID, exists := claims["tenant_id"]
    if !exists {
      tenantID, exists = claims["tenantid"]
    }

    if !exists {
      c.JSON(http.StatusUnautorized, gin.H{"error": "tenant_id is missing from token claims"})
      c.Abort()
      return
    }

    c.Set("tenant_id", fmt.Sprintf("%v", tenantID))

    c.Next()
	}
}