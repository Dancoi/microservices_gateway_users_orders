package main

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/glebarez/sqlite"
	"gorm.io/gorm"
)

func setupTestServer(t *testing.T) (*gin.Engine, *gorm.DB) {
	os.Setenv("JWT_SECRET", "test-secret")
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatalf("failed open db: %v", err)
	}
	if err := db.AutoMigrate(&User{}); err != nil {
		t.Fatalf("migrate failed: %v", err)
	}
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(RequestIDMiddleware())
	r.Use(CORSMiddleware())
	RegisterHandlers(r, db)
	return r, db
}

func TestRegisterLoginProfileFlow(t *testing.T) {
	r, _ := setupTestServer(t)

	// Register
	body := map[string]string{"email": "u1@example.com", "password": "password", "name": "User 1"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", w.Code, w.Body.String())
	}

	// Login
	cred := map[string]string{"email": "u1@example.com", "password": "password"}
	cb, _ := json.Marshal(cred)
	req = httptest.NewRequest(http.MethodPost, "/v1/users/login", bytes.NewReader(cb))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	token := data["token"].(string)

	// Access protected path without token
	req = httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 when no token, got %d", w.Code)
	}

	// Access with token
	req = httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d body=%s", w.Code, w.Body.String())
	}
}

func TestDuplicateRegistrationAndUpdateProfile(t *testing.T) {
	r, _ := setupTestServer(t)

	// First register
	body := map[string]string{"email": "dup@example.com", "password": "password", "name": "Dup"}
	b, _ := json.Marshal(body)
	req := httptest.NewRequest(http.MethodPost, "/v1/users/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 got %d body=%s", w.Code, w.Body.String())
	}

	// Duplicate register
	req = httptest.NewRequest(http.MethodPost, "/v1/users/register", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusConflict {
		t.Fatalf("expected 409 for duplicate, got %d body=%s", w.Code, w.Body.String())
	}

	// Login
	cred := map[string]string{"email": "dup@example.com", "password": "password"}
	cb, _ := json.Marshal(cred)
	req = httptest.NewRequest(http.MethodPost, "/v1/users/login", bytes.NewReader(cb))
	req.Header.Set("Content-Type", "application/json")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("login failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	data := resp["data"].(map[string]interface{})
	token := data["token"].(string)

	// Update profile (change name)
	updates := map[string]string{"name": "Dup2"}
	ub, _ := json.Marshal(updates)
	req = httptest.NewRequest(http.MethodPut, "/v1/users/me", bytes.NewReader(ub))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("update failed: %d %s", w.Code, w.Body.String())
	}

	// Get profile and assert name updated
	req = httptest.NewRequest(http.MethodGet, "/v1/users/me", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get profile failed: %d %s", w.Code, w.Body.String())
	}
	var gresp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &gresp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	gdata := gresp["data"].(map[string]interface{})
	if gdata["name"].(string) != "Dup2" {
		t.Fatalf("expected name updated to Dup2, got %v", gdata["name"])
	}
}
