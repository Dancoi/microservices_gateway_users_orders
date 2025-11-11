package main

import (
	"fmt"
	"io"
	stdlog "log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/time/rate"
)

var jwtSecret = []byte(getEnv("JWT_SECRET", "dev-secret"))

func getEnv(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func main() {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestID())
	r.Use(CORSMiddleware())

	// rate limiter per IP
	limiter := rate.NewLimiter(5, 20)

	// proxy routes
	usersURL := getEnv("USERS_URL", "http://service_users:8000")
	ordersURL := getEnv("ORDERS_URL", "http://service_orders:8000")

	v1 := r.Group("/v1")

	v1.Any("/users/*path", func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(429, gin.H{"success": false, "error": gin.H{"code": "rate_limited", "message": "rate limit"}})
			return
		}
		proxyTo(c, usersURL)
	})

	v1.Any("/orders/*path", func(c *gin.Context) {
		if !limiter.Allow() {
			c.AbortWithStatusJSON(429, gin.H{"success": false, "error": gin.H{"code": "rate_limited", "message": "rate limit"}})
			return
		}
		proxyTo(c, ordersURL)
	})

	port := getEnv("PORT", "8000")
	addr := fmt.Sprintf(":%s", port)
	stdlog.Printf("api_gateway running %s", addr)
	if err := r.Run(addr); err != nil {
		stdlog.Fatalf("failed: %v", err)
	}
}

func proxyTo(c *gin.Context, backend string) {
	// simple reverse proxy: forward request to backend preserving method, headers and body
	client := &http.Client{Timeout: 10 * time.Second}
	req, err := http.NewRequest(c.Request.Method, backend+c.Request.RequestURI, c.Request.Body)
	if err != nil {
		c.JSON(500, gin.H{"success": false})
		return
	}
	// copy headers
	for k, v := range c.Request.Header {
		for _, vv := range v {
			req.Header.Add(k, vv)
		}
	}
	// propagate X-Request-ID
	if req.Header.Get("X-Request-ID") == "" {
		req.Header.Set("X-Request-ID", c.GetString("X-Request-ID"))
	}
	// JWT validation on protected paths (simple: only /users/register and /users/login public)
	if isProtected(c.Request.RequestURI) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.AbortWithStatusJSON(401, gin.H{"success": false, "error": gin.H{"code": "unauthorized", "message": "token required"}})
			return
		}
		if !validateJWT(auth) {
			c.AbortWithStatusJSON(401, gin.H{"success": false, "error": gin.H{"code": "unauthorized", "message": "invalid token"}})
			return
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		c.JSON(502, gin.H{"success": false, "error": gin.H{"code": "bad_gateway", "message": err.Error()}})
		return
	}
	defer resp.Body.Close()

	// copy response
	for k, v := range resp.Header {
		for _, vv := range v {
			c.Writer.Header().Add(k, vv)
		}
	}
	c.Status(resp.StatusCode)
	io.Copy(c.Writer, resp.Body)
}

func isProtected(uri string) bool {
	// naive: allow register/login without token
	if strings.HasPrefix(uri, "/v1/users/register") || strings.HasPrefix(uri, "/v1/users/login") {
		return false
	}
	return true
}

func validateJWT(authHeader string) bool {
	if !strings.HasPrefix(authHeader, "Bearer ") {
		return false
	}
	tokenStr := strings.TrimPrefix(authHeader, "Bearer ")
	token, err := jwt.Parse(tokenStr, func(t *jwt.Token) (interface{}, error) {
		if t.Method.Alg() != jwt.SigningMethodHS256.Alg() {
			return nil, jwt.ErrTokenUnverifiable
		}
		return jwtSecret, nil
	})
	if err != nil || !token.Valid {
		return false
	}
	return true
}

func RequestID() gin.HandlerFunc {
	return func(c *gin.Context) {
		rid := c.GetHeader("X-Request-ID")
		if rid == "" {
			rid = fmt.Sprintf("%d", time.Now().UnixNano())
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
