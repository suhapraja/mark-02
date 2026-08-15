package db

import (
	"log"

	"github.com/suhapraja/mark-02/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func Connect(databaseURL string) *gorm.DB {
	conn, err := gorm.Open(postgres.Open(databaseURL), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}
	return conn
}

// AutoMigrate creates/updates tables based on the model structs.
// For a project this size, GORM's auto-migration is enough — no need
// for a separate migration tool.
func AutoMigrate(conn *gorm.DB) error {
	return conn.AutoMigrate(
		&models.Car{},
		&models.Driver{},
		&models.Order{},
		&models.LocationLog{},
	)
}
