package main

import (
	"fmt"
	stdlog "log"
	"os"

	"github.com/gin-gonic/gin"
	zlog "github.com/rs/zerolog/log"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := os.Getenv("DATABASE_DSN")
	if dsn == "" {
		// default for docker-compose
		dsn = "host=postgres user=postgres password=postgres dbname=app_db port=5432 sslmode=disable"
	}

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		stdlog.Fatalf("failed to connect database: %v", err)
	}

	// migrate
	if err := migrate(db); err != nil {
		stdlog.Fatalf("migrate failed: %v", err)
	}

	r := gin.New()
	r.Use(gin.Recovery())

	// middleware
	r.Use(RequestIDMiddleware())
	r.Use(CORSMiddleware())

	// handlers
	RegisterHandlers(r, db)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8000"
	}
	addr := fmt.Sprintf(":%s", port)
	zlog.Info().Msgf("service_users running on %s", addr)
	if err := r.Run(addr); err != nil {
		zlog.Fatal().Err(err).Msg("server failed")
	}
}
