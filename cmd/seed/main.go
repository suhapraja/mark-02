// Seed loads cars and drivers from JSON files into the database.
// Run once during initial setup, and again any time the fixed
// car/driver list changes.
//
// Usage:
//
//	go run ./cmd/seed -cars=seed/cars.json -drivers=seed/drivers.json
package main

import (
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/suhapraja/mark-02/internal/config"
	"github.com/suhapraja/mark-02/internal/db"
	"github.com/suhapraja/mark-02/internal/models"
)

type carSeed struct {
	PlateNumber string `json:"plate_number"`
	Model       string `json:"model"`
}

type driverSeed struct {
	Name  string `json:"name"`
	Phone string `json:"phone"` // format: 62812xxxxxxx (no "+", no spaces)
}

func main() {
	carsPath := flag.String("cars", "seed/cars.json", "path to cars JSON file")
	driversPath := flag.String("drivers", "seed/drivers.json", "path to drivers JSON file")
	flag.Parse()

	cfg := config.Load()
	conn := db.Connect(cfg.DatabaseURL)
	if err := db.AutoMigrate(conn); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	var cars []carSeed
	if err := readJSON(*carsPath, &cars); err != nil {
		log.Fatalf("failed to read cars file: %v", err)
	}
	for _, c := range cars {
		car := models.Car{
			PlateNumber: c.PlateNumber,
			Model:       c.Model,
			Status:      models.CarAvailable,
		}
		if err := conn.Where(models.Car{PlateNumber: c.PlateNumber}).
			FirstOrCreate(&car).Error; err != nil {
			log.Fatalf("failed to seed car %s: %v", c.PlateNumber, err)
		}
	}
	log.Printf("seeded %d cars", len(cars))

	var drivers []driverSeed
	if err := readJSON(*driversPath, &drivers); err != nil {
		log.Fatalf("failed to read drivers file: %v", err)
	}
	for _, d := range drivers {
		driver := models.Driver{
			Name:   d.Name,
			Phone:  d.Phone,
			Status: models.DriverAvailable,
		}
		if err := conn.Where(models.Driver{Phone: d.Phone}).
			FirstOrCreate(&driver).Error; err != nil {
			log.Fatalf("failed to seed driver %s: %v", d.Name, err)
		}
	}
	log.Printf("seeded %d drivers", len(drivers))
}

func readJSON(path string, out any) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewDecoder(f).Decode(out)
}
