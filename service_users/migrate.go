package main

import "gorm.io/gorm"

func migrate(db *gorm.DB) error {
	return db.AutoMigrate(&User{})
}
