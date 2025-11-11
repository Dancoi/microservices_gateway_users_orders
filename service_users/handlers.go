package main

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type registerRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required,min=6"`
	Name     string `json:"name" binding:"required"`
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

func RegisterHandlers(r *gin.Engine, db *gorm.DB) {
	v1 := r.Group("/v1")

	users := v1.Group("/users")
	{
		users.POST("/register", func(c *gin.Context) {
			var req registerRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "invalid_input", "message": err.Error()}})
				return
			}
			// check email
			var exist User
			if err := db.Where("email = ?", strings.ToLower(req.Email)).First(&exist).Error; err == nil {
				c.JSON(http.StatusConflict, gin.H{"success": false, "error": gin.H{"code": "email_exists", "message": "Email already registered"}})
				return
			} else if err != gorm.ErrRecordNotFound {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "db_error", "message": "DB error"}})
				return
			}

			hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
			u := User{Email: strings.ToLower(req.Email), Password: string(hash), Name: req.Name, Roles: pqStringArray{"user"}}
			if err := db.Create(&u).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "db_error", "message": "cannot create user"}})
				return
			}
			c.JSON(http.StatusCreated, gin.H{"success": true, "data": gin.H{"id": u.ID}})
		})

		users.POST("/login", func(c *gin.Context) {
			var req loginRequest
			if err := c.ShouldBindJSON(&req); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "invalid_input", "message": err.Error()}})
				return
			}
			var u User
			if err := db.Where("email = ?", strings.ToLower(req.Email)).First(&u).Error; err != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "invalid_credentials", "message": "invalid credentials"}})
				return
			}
			if bcrypt.CompareHashAndPassword([]byte(u.Password), []byte(req.Password)) != nil {
				c.JSON(http.StatusUnauthorized, gin.H{"success": false, "error": gin.H{"code": "invalid_credentials", "message": "invalid credentials"}})
				return
			}
			token, err := GenerateJWT(u.ID.String(), []string(u.Roles))
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "token_error", "message": "cannot generate token"}})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true, "data": gin.H{"token": token}})
		})

		users.GET("/", AuthMiddleware(db, true), func(c *gin.Context) {
			// admin only list
			// pagination
			var users []User
			limit := 20
			offset := 0
			db.Limit(limit).Offset(offset).Find(&users)
			c.JSON(http.StatusOK, gin.H{"success": true, "data": users})
		})

		users.GET("/me", AuthMiddleware(db, false), func(c *gin.Context) {
			uid := c.GetString("user_id")
			var u User
			if err := db.Where("id = ?", uid).First(&u).Error; err != nil {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "not_found", "message": "user not found"}})
				return
			}
			u.Password = ""
			c.JSON(http.StatusOK, gin.H{"success": true, "data": u})
		})

		users.PUT("/me", AuthMiddleware(db, false), func(c *gin.Context) {
			uid := c.GetString("user_id")
			var updates map[string]interface{}
			if err := c.ShouldBindJSON(&updates); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "invalid_input", "message": err.Error()}})
				return
			}
			// disallow role changes and password here
			delete(updates, "roles")
			delete(updates, "password")
			if err := db.Model(&User{}).Where("id = ?", uid).Updates(updates).Error; err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "db_error", "message": "cannot update"}})
				return
			}
			c.JSON(http.StatusOK, gin.H{"success": true})
		})
	}
}
