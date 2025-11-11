package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Local minimal User model used only for tests in this package.
type User struct {
	ID    uuid.UUID `gorm:"type:uuid;primaryKey" json:"id"`
	Email string    `gorm:"uniqueIndex;not null" json:"email"`
	Name  string    `json:"name"`
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}

func setupOrdersTest(t *testing.T) (*gin.Engine, *gorm.DB, string) {
	os.Setenv("JWT_SECRET", "test-secret")
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	if err := db.AutoMigrate(&User{}, &Order{}); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestIDMiddleware())
	r.Use(CORSMiddleware())
	RegisterOrderHandlers(r, db)

	// create user directly in DB and generate token
	user := User{Email: "o1@example.com", Name: "Owner"}
	if err := db.Create(&user).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	// generate JWT token
	claims := jwt.MapClaims{
		"sub":   user.ID.String(),
		"roles": []string{},
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	tokenStr, err := tokenObj.SignedString(jwtSecretOrders)
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	return r, db, tokenStr
}

func TestCreateAndGetOrder(t *testing.T) {
	r, _, token := setupOrdersTest(t)

	// create with items as JSON string because createOrderReq expects Items string
	order := map[string]interface{}{"items": "[]", "total": 10.5}
	b, _ := json.Marshal(order)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("create order failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	id := data["id"].(string)

	// get order
	req = httptest.NewRequest(http.MethodGet, "/v1/orders/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get order failed: %d %s", w.Code, w.Body.String())
	}
}
