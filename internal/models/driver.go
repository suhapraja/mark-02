package models

import "time"

type DriverStatus string

const (
	DriverAvailable DriverStatus = "available"
	DriverOnTrip    DriverStatus = "on_trip"
)

type Driver struct {
	ID           uint         `gorm:"primaryKey" json:"id"`
	Name         string       `gorm:"not null" json:"name"`
	Phone        string       `gorm:"uniqueIndex;not null" json:"phone"` // WhatsApp number, e.g. 62812xxxxxxx
	LastLocation string       `json:"last_location"`
	Status       DriverStatus `gorm:"type:varchar(20);not null;default:available" json:"status"`
	CreatedAt    time.Time    `json:"created_at"`
	UpdatedAt    time.Time    `json:"updated_at"`
}
