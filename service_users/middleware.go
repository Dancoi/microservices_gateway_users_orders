package main

import (
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gorm.io/gorm"
)

var jwtSecret = []byte(getEnv("JWT_SECRET", "dev-secret"))

func getEnv(key, def string) string {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	return v
}

func GenerateJWT(userID string, roles []string) (string, error) {
	claims := jwt.MapClaims{
		"sub":   userID,
		"roles": roles,
		"exp":   time.Now().Add(24 * time.Hour).Unix(),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// AuthMiddleware verifies JWT and sets user context. If adminOnly==true, requires role 'admin'
func AuthMiddleware(db *gorm.DB, adminOnly bool) gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" || !strings.HasPrefix(auth, "Bearer ") {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "unauthorized", "message": "missing token"}})
			return
		}
		tokenStr := strings.TrimPrefix(auth, "Bearer ")
		token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
			if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
				return nil, jwt.ErrTokenUnverifiable
			}
			return jwtSecret, nil
		})
		if err != nil || !token.Valid {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "unauthorized", "message": "invalid token"}})
			return
		}
		claims, ok := token.Claims.(jwt.MapClaims)
		if !ok {
			c.AbortWithStatusJSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "unauthorized", "message": "invalid token claims"}})
			return
		}
		sub, _ := claims["sub"].(string)
		rolesIface := claims["roles"]
		var roles []string
		if rolesSlice, ok := rolesIface.([]interface{}); ok {
			for _, r := range rolesSlice {
				if s, ok := r.(string); ok {
					roles = append(roles, s)
				}
			}
		}
		if adminOnly {
			found := false
			for _, r := range roles {
				if r == "admin" {
					found = true
					break
				}
			}
			if !found {
				c.AbortWithStatusJSON(http.StatusForbidden, gin.H{"success": false, "error": gin.H{"code": "forbidden", "message": "admin role required"}})
				return
			}
		}
		c.Set("user_id", sub)
		c.Set("roles", roles)
		c.Next()
	}
}

func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = generateRequestID()
		}
		c.Writer.Header().Set("X-Request-ID", rid)
		c.Set("X-Request-ID", rid)
		c.Next()
	}
}

func CORSMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Writer.Header().Set("Access-Control-Allow-Origin", "*")
		c.Writer.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Writer.Header().Set("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, X-Request-ID")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	}
}

func generateRequestID() string {
	return uuid.New().String()
}

func LoggingMiddleware() gin.HandlerFunc {
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnixMicro
	logger := log.Logger
	return func(c *gin.Context) {
		start := time.Now()
		rid := c.GetString("X-Request-ID")
		c.Next()
		latency := time.Since(start)
		logger.Info().Str("rid", rid).
			Str("method", c.Request.Method).
			Str("path", c.Request.RequestURI).
			Int("status", c.Writer.Status()).
			Int64("latency_ms", latency.Milliseconds()).
			Msg("http_request")
	}
}
