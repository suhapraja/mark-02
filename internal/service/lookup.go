package service

import (
	"fmt"
	"strings"

	"github.com/suhapraja/mark-02/internal/models"
	"gorm.io/gorm"
)

// FindCarByQuery looks up a car by plate number, model, or both together
// (e.g. "Avanza B1234"), case-insensitive and forgiving of spacing in the
// plate number. Every word in the query must appear in either the model
// or the plate number for a car to match. Returns an error if zero or
// more than one match is found, since booking needs to be unambiguous.
func FindCarByQuery(db *gorm.DB, query string) (*models.Car, error) {
	var cars []models.Car
	if err := db.Find(&cars).Error; err != nil {
		return nil, err
	}

	tokens := strings.Fields(strings.ToLower(query))
	var matches []models.Car
	for _, c := range cars {
		modelLower := strings.ToLower(c.Model)
		plateLower := strings.ToLower(strings.ReplaceAll(c.PlateNumber, " ", ""))

		matched := true
		for _, t := range tokens {
			tNorm := strings.ReplaceAll(t, " ", "")
			if !strings.Contains(modelLower, t) && !strings.Contains(plateLower, tNorm) {
				matched = false
				break
			}
		}
		if matched {
			matches = append(matches, c)
		}
	}

	switch len(matches) {
	case 0:
		return nil, fmt.Errorf("mobil tidak ditemukan: %q", query)
	case 1:
		return &matches[0], nil
	default:
		return nil, fmt.Errorf("lebih dari satu mobil cocok dengan %q, gunakan plat nomor lengkap", query)
	}
}

// FindDriverByQuery looks up a driver by name, case-insensitive, partial match.
func FindDriverByQuery(db *gorm.DB, query string) (*models.Driver, error) {
	var drivers []models.Driver
	like := "%" + query + "%"
	if err := db.Where("name ILIKE ?", like).Find(&drivers).Error; err != nil {
		return nil, err
	}
	switch len(drivers) {
	case 0:
		return nil, fmt.Errorf("driver tidak ditemukan: %q", query)
	case 1:
		return &drivers[0], nil
	default:
		return nil, fmt.Errorf("lebih dari satu driver cocok dengan %q, gunakan nama lengkap", query)
	}
}

// FindDriverByPhone looks up a driver by their WhatsApp phone number.
func FindDriverByPhone(db *gorm.DB, phone string) (*models.Driver, error) {
	var driver models.Driver
	if err := db.Where("phone = ?", phone).First(&driver).Error; err != nil {
		return nil, err
	}
	return &driver, nil
}
