package main

import "gorm.io/gorm"

func migrateOrders(db *gorm.DB) error {
	return db.AutoMigrate(&Order{})
}
