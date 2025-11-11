package main

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

type createOrderReq struct {
	Items string  `json:"items" binding:"required"`
	Total float64 `json:"total" binding:"required"`
}

func RegisterOrderHandlers(r *gin.Engine, db *gorm.DB) {
	v1 := r.Group("/v1")
	ord := v1.Group("/orders")

	ord.POST("/", OrderAuthMiddleware(), func(c *gin.Context) {
		var req createOrderReq
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "invalid_input", "message": err.Error()}})
			return
		}
		uid := c.GetString("user_id")
		parsed, _ := uuid.Parse(uid)
		// check user exists
		var cnt int64
		db.Table("users").Where("id = ?", parsed).Count(&cnt)
		if cnt == 0 {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "user_not_found", "message": "user not found"}})
			return
		}
		o := Order{UserID: parsed, Items: req.Items, Total: req.Total}
		if err := db.Create(&o).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "db_error", "message": "cannot create order"}})
			return
		}
		// domain event could be published here (placeholder)
		c.JSON(http.StatusCreated, gin.H{"success": true, "data": o})
	})

	ord.GET("/", OrderAuthMiddleware(), func(c *gin.Context) {
		// list orders for current user
		uid := c.GetString("user_id")
		parsed, _ := uuid.Parse(uid)
		var orders []Order
		db.Where("user_id = ?", parsed).Find(&orders)
		c.JSON(http.StatusOK, gin.H{"success": true, "data": orders})
	})

	ord.GET("/:orderId", OrderAuthMiddleware(), func(c *gin.Context) {
		idStr := c.Param("orderId")
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "invalid_id", "message": "invalid id"}})
			return
		}
		var o Order
		if err := db.First(&o, "id = ?", id).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "not_found", "message": "order not found"}})
				return
			}
			c.JSON(http.StatusInternalServerError, gin.H{"success": false, "error": gin.H{"code": "db_error", "message": "db error"}})
			return
		}
		// check owner or admin
		uid := c.GetString("user_id")
		if o.UserID.String() != uid {
			// check roles
			rolesIface, _ := c.Get("roles")
			if roles, ok := rolesIface.([]string); !ok || !contains(roles, "admin") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": gin.H{"code": "forbidden", "message": "not allowed"}})
				return
			}
		}
		c.JSON(http.StatusOK, gin.H{"success": true, "data": o})
	})

	ord.PUT("/:orderId/status", OrderAuthMiddleware(), func(c *gin.Context) {
		idStr := c.Param("orderId")
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "invalid_id", "message": "invalid id"}})
			return
		}
		var body struct {
			Status string `json:"status"`
		}
		if err := c.ShouldBindJSON(&body); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "invalid_input", "message": err.Error()}})
			return
		}
		var o Order
		if err := db.First(&o, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "not_found", "message": "order not found"}})
			return
		}
		// only owner or admin
		uid := c.GetString("user_id")
		if o.UserID.String() != uid {
			rolesIface, _ := c.Get("roles")
			if roles, ok := rolesIface.([]string); !ok || !contains(roles, "admin") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": gin.H{"code": "forbidden", "message": "not allowed"}})
				return
			}
		}
		o.Status = body.Status
		db.Save(&o)
		// domain event placeholder
		c.JSON(http.StatusOK, gin.H{"success": true, "data": o})
	})

	ord.DELETE("/:orderId", OrderAuthMiddleware(), func(c *gin.Context) {
		idStr := c.Param("orderId")
		id, err := uuid.Parse(idStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": gin.H{"code": "invalid_id", "message": "invalid id"}})
			return
		}
		var o Order
		if err := db.First(&o, "id = ?", id).Error; err != nil {
			c.JSON(http.StatusNotFound, gin.H{"success": false, "error": gin.H{"code": "not_found", "message": "order not found"}})
			return
		}
		uid := c.GetString("user_id")
		if o.UserID.String() != uid {
			rolesIface, _ := c.Get("roles")
			if roles, ok := rolesIface.([]string); !ok || !contains(roles, "admin") {
				c.JSON(http.StatusForbidden, gin.H{"success": false, "error": gin.H{"code": "forbidden", "message": "not allowed"}})
				return
			}
		}
		db.Delete(&o)
		c.JSON(http.StatusOK, gin.H{"success": true})
	})
}

func contains(arr []string, v string) bool {
	for _, s := range arr {
		if s == v {
			return true
		}
	}
	return false
}
