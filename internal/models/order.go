package models

import "time"

type OrderStatus string

const (
	OrderActive    OrderStatus = "active"
	OrderCompleted OrderStatus = "completed"
	OrderCancelled OrderStatus = "cancelled"
)

type Order struct {
	ID              uint        `gorm:"primaryKey" json:"id"`
	CarID           uint        `gorm:"not null;index" json:"car_id"`
	Car             Car         `json:"car"`
	DriverID        uint        `gorm:"not null;index" json:"driver_id"`
	Driver          Driver      `json:"driver"`
	CustomerName    string      `gorm:"not null" json:"customer_name"`
	CustomerPhone   string      `json:"customer_phone"`
	PickupDatetime  time.Time   `gorm:"not null" json:"pickup_datetime"`
	ReturnDatetime  time.Time   `gorm:"not null" json:"return_datetime"`
	DestinationCity string      `json:"destination_city"`
	Status          OrderStatus `gorm:"type:varchar(20);not null;default:active" json:"status"`
	CreatedAt       time.Time   `json:"created_at"`
	LastEditedAt    time.Time   `json:"last_edited_at"`

	// Optional fields mirroring the columns of the spreadsheet this bot
	// replaces, so the Excel export matches what the business already
	// reads. All are optional — a booking without them still works.
	Pemesan     string `json:"pemesan"`      // booking agent, e.g. "Ria DX"
	PickupPoint string `json:"pickup_point"` // "Jptan", e.g. "Soetta", "Home"
	Partner     string `json:"partner"`      // "Rekanan", set when the car is subcontracted
	Notes       string `json:"notes"`        // "Keterangan", free text
}
