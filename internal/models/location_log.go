package models

import "time"

// LocationLog keeps a history of driver location updates, useful for
// auditing where a driver has been over time, beyond just the latest
// value stored on Driver.LastLocation.
type LocationLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	DriverID  uint      `gorm:"not null;index" json:"driver_id"`
	Location  string    `gorm:"not null" json:"location"`
	CreatedAt time.Time `json:"created_at"`
}
