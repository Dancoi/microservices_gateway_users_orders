package main

import (
	"database/sql/driver"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type User struct {
	ID        uuid.UUID     `gorm:"type:uuid;primaryKey" json:"id"`
	Email     string        `gorm:"uniqueIndex;not null" json:"email"`
	Password  string        `gorm:"not null" json:"-"`
	Name      string        `json:"name"`
	Roles     pqStringArray `gorm:"type:text[];default:'{user}'" json:"roles"`
	CreatedAt time.Time     `json:"created_at"`
	UpdatedAt time.Time     `json:"updated_at"`
}

// pqStringArray is a helper type for postgres text[] via simple parsing
type pqStringArray []string

func (a *pqStringArray) Scan(value interface{}) error {
	if value == nil {
		*a = []string{}
		return nil
	}
	var s string
	switch v := value.(type) {
	case string:
		s = v
	case []byte:
		s = string(v)
	default:
		return fmt.Errorf("unsupported type for pqStringArray: %T", value)
	}
	// expect format like {a,b}
	s = strings.TrimPrefix(strings.TrimSuffix(s, "}"), "{")
	if s == "" {
		*a = []string{}
		return nil
	}
	parts := strings.Split(s, ",")
	for i := range parts {
		parts[i] = strings.Trim(parts[i], `"`)
	}
	*a = parts
	return nil
}

func valueStringArray(arr []string) (driver.Value, error) {
	if arr == nil || len(arr) == 0 {
		return "{}", nil
	}
	// join with commas and wrap
	for i := range arr {
		if strings.ContainsAny(arr[i], ",{}\" ") {
			// naive escaping
			arr[i] = `"` + strings.ReplaceAll(arr[i], `"`, `\"`) + `"`
		}
	}
	return "{" + strings.Join(arr, ",") + "}", nil
}

func (u *User) BeforeCreate(tx *gorm.DB) (err error) {
	if u.ID == uuid.Nil {
		u.ID = uuid.New()
	}
	return nil
}
