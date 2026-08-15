package models

import "time"

type CarStatus string

const (
	CarAvailable   CarStatus = "available"
	CarOnTrip      CarStatus = "on_trip"
	CarMaintenance CarStatus = "maintenance"
)

type Car struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	PlateNumber string    `gorm:"uniqueIndex;not null" json:"plate_number"`
	Model       string    `gorm:"not null" json:"model"`
	Status      CarStatus `gorm:"type:varchar(20);not null;default:available" json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
