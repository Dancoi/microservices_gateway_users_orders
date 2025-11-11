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

func setupOrdersTestEngine(t *testing.T) (*gin.Engine, *gorm.DB) {
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
	return r, db
}

func createTokenForUser(id uuid.UUID) (string, error) {
	claims := jwt.MapClaims{
		"sub":   id.String(),
		"roles": []string{},
		"exp":   time.Now().Add(time.Hour).Unix(),
	}
	tokenObj := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tokenObj.SignedString(jwtSecretOrders)
}

func TestOrderStatusChangeAndDelete(t *testing.T) {
	r, db := setupOrdersTestEngine(t)

	// create user directly
	uid := uuid.New()
	u := User{ID: uid, Email: "owner@example.com", Name: "Owner"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, err := createTokenForUser(uid)
	if err != nil {
		t.Fatalf("token sign: %v", err)
	}

	// create order
	order := map[string]interface{}{"items": "[]", "total": 5.0}
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

	// change status to cancelled
	statusBody := map[string]string{"status": "cancelled"}
	sb, _ := json.Marshal(statusBody)
	req = httptest.NewRequest(http.MethodPut, "/v1/orders/"+id+"/status", bytes.NewReader(sb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("status change failed: %d %s", w.Code, w.Body.String())
	}

	// verify status changed to cancelled via GET
	req = httptest.NewRequest(http.MethodGet, "/v1/orders/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("get after status failed: %d %s", w.Code, w.Body.String())
	}
	var got map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &got); err != nil {
		t.Fatalf("invalid json after status: %v", err)
	}
	d := got["data"].(map[string]interface{})
	if d["status"].(string) != "cancelled" {
		t.Fatalf("expected status cancelled, got %v", d["status"])
	}

	// delete order
	req = httptest.NewRequest(http.MethodDelete, "/v1/orders/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("delete failed: %d %s", w.Code, w.Body.String())
	}

	// get should be 404
	req = httptest.NewRequest(http.MethodGet, "/v1/orders/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404 after delete, got %d", w.Code)
	}
}

func TestForbiddenModifyOtherOrder(t *testing.T) {
	r, db := setupOrdersTestEngine(t)

	// create owner
	ownerID := uuid.New()
	owner := User{ID: ownerID, Email: "owner2@example.com", Name: "Owner2"}
	if err := db.Create(&owner).Error; err != nil {
		t.Fatalf("create owner: %v", err)
	}
	ownerToken, _ := createTokenForUser(ownerID)

	// create other user
	otherID := uuid.New()
	other := User{ID: otherID, Email: "other@example.com", Name: "Other"}
	if err := db.Create(&other).Error; err != nil {
		t.Fatalf("create other: %v", err)
	}
	otherToken, _ := createTokenForUser(otherID)

	// owner creates order
	order := map[string]interface{}{"items": "[]", "total": 7.0}
	b, _ := json.Marshal(order)
	req := httptest.NewRequest(http.MethodPost, "/v1/orders/", bytes.NewReader(b))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+ownerToken)
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

	// other tries to change status
	statusBody := map[string]string{"status": "shipped"}
	sb, _ := json.Marshal(statusBody)
	req = httptest.NewRequest(http.MethodPut, "/v1/orders/"+id+"/status", bytes.NewReader(sb))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+otherToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden when other modifies, got %d body=%s", w.Code, w.Body.String())
	}

	// other tries to delete
	req = httptest.NewRequest(http.MethodDelete, "/v1/orders/"+id, nil)
	req.Header.Set("Authorization", "Bearer "+otherToken)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected forbidden on delete by other, got %d", w.Code)
	}
}

func TestOrdersPagination(t *testing.T) {
	r, db := setupOrdersTestEngine(t)

	// create user
	uid := uuid.New()
	u := User{ID: uid, Email: "pag@example.com", Name: "Pager"}
	if err := db.Create(&u).Error; err != nil {
		t.Fatalf("create user: %v", err)
	}
	token, _ := createTokenForUser(uid)

	// create 25 orders
	for i := 0; i < 25; i++ {
		order := map[string]interface{}{"items": "[]", "total": 1.0}
		b, _ := json.Marshal(order)
		req := httptest.NewRequest(http.MethodPost, "/v1/orders/", bytes.NewReader(b))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		if w.Code != http.StatusCreated {
			t.Fatalf("create order failed: %d %s", w.Code, w.Body.String())
		}
	}

	// request page 2 size 10 -> items 10, total 25, total_pages 3
	req := httptest.NewRequest(http.MethodGet, "/v1/orders/?page=2&size=10", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Fatalf("pagination request failed: %d %s", w.Code, w.Body.String())
	}
	var resp map[string]interface{}
	if err := json.Unmarshal(w.Body.Bytes(), &resp); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	meta := resp["meta"].(map[string]interface{})
	if int(meta["total"].(float64)) != 25 {
		t.Fatalf("expected total 25, got %v", meta["total"])
	}
	if int(meta["page"].(float64)) != 2 {
		t.Fatalf("expected page 2, got %v", meta["page"])
	}
	if int(meta["size"].(float64)) != 10 {
		t.Fatalf("expected size 10, got %v", meta["size"])
	}
	if int(meta["total_pages"].(float64)) != 3 {
		t.Fatalf("expected total_pages 3, got %v", meta["total_pages"])
	}
	data := resp["data"].([]interface{})
	if len(data) != 10 {
		t.Fatalf("expected 10 items on page, got %d", len(data))
	}
}
