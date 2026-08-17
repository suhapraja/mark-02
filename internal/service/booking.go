package service

import (
	"fmt"
	"time"

	"github.com/suhapraja/mark-02/internal/models"
	"github.com/suhapraja/mark-02/internal/parser"
	"gorm.io/gorm"
)

type BookingService struct {
	DB           *gorm.DB
	Availability *AvailabilityService
}

func NewBookingService(db *gorm.DB, availability *AvailabilityService) *BookingService {
	return &BookingService{DB: db, Availability: availability}
}

type CreateBookingInput struct {
	CarQuery      string
	DriverQuery   string
	CustomerName  string
	CustomerPhone string
	Destination   string
	Pemesan       string
	PickupPoint   string
	Partner       string
	Notes         string
	Start         time.Time
	End           time.Time
}

func (s *BookingService) CreateBooking(in CreateBookingInput) (*models.Order, error) {
	car, err := FindCarByQuery(s.DB, in.CarQuery)
	if err != nil {
		return nil, err
	}
	if car.Status == models.CarMaintenance {
		return nil, fmt.Errorf("mobil %s sedang dalam perbaikan (maintenance)", car.PlateNumber)
	}

	driver, err := FindDriverByQuery(s.DB, in.DriverQuery)
	if err != nil {
		return nil, err
	}

	conflict, err := s.Availability.HasConflict(car.ID, driver.ID, in.Start, in.End, 0)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, fmt.Errorf(
			"jadwal bentrok: mobil %s atau driver %s sudah ada order aktif di rentang waktu tersebut",
			car.PlateNumber, driver.Name)
	}

	order := &models.Order{
		CarID:           car.ID,
		DriverID:        driver.ID,
		CustomerName:    in.CustomerName,
		CustomerPhone:   in.CustomerPhone,
		PickupDatetime:  in.Start,
		ReturnDatetime:  in.End,
		DestinationCity: in.Destination,
		Pemesan:         in.Pemesan,
		PickupPoint:     in.PickupPoint,
		Partner:         in.Partner,
		Notes:           in.Notes,
		Status:          models.OrderActive,
		CreatedAt:       time.Now().In(parser.JakartaLocation),
		LastEditedAt:    time.Now().In(parser.JakartaLocation),
	}

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Create(order).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Car{}).Where("id = ?", car.ID).
			Update("status", models.CarOnTrip).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Driver{}).Where("id = ?", driver.ID).
			Update("status", models.DriverOnTrip).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	// Reload with associations for the confirmation message / driver notify.
	if err := s.DB.Preload("Car").Preload("Driver").First(order, order.ID).Error; err != nil {
		return nil, err
	}

	return order, nil
}

func (s *BookingService) CancelBooking(orderID uint) (*models.Order, error) {
	var order models.Order
	if err := s.DB.Preload("Car").Preload("Driver").First(&order, orderID).Error; err != nil {
		return nil, fmt.Errorf("order #%d tidak ditemukan", orderID)
	}
	if order.Status != models.OrderActive {
		return nil, fmt.Errorf("order #%d sudah %s, tidak bisa dibatalkan", orderID, order.Status)
	}

	err := s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&order).Update("status", models.OrderCancelled).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Car{}).Where("id = ?", order.CarID).
			Update("status", models.CarAvailable).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Driver{}).Where("id = ?", order.DriverID).
			Update("status", models.DriverAvailable).Error; err != nil {
			return err
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return &order, nil
}

type EditBookingInput struct {
	OrderID uint
	Field   string // "driver" | "mobil" | "waktu"
	Value   string // new driver/car name, unused for "waktu"
	Start   time.Time
	End     time.Time
}

func (s *BookingService) EditBooking(in EditBookingInput) (*models.Order, error) {
	var order models.Order
	if err := s.DB.First(&order, in.OrderID).Error; err != nil {
		return nil, fmt.Errorf("order #%d tidak ditemukan", in.OrderID)
	}
	if order.Status != models.OrderActive {
		return nil, fmt.Errorf("order #%d sudah %s, tidak bisa diubah", in.OrderID, order.Status)
	}

	newCarID := order.CarID
	newDriverID := order.DriverID
	newStart := order.PickupDatetime
	newEnd := order.ReturnDatetime

	switch in.Field {
	case "mobil":
		car, err := FindCarByQuery(s.DB, in.Value)
		if err != nil {
			return nil, err
		}
		newCarID = car.ID
	case "driver":
		driver, err := FindDriverByQuery(s.DB, in.Value)
		if err != nil {
			return nil, err
		}
		newDriverID = driver.ID
	case "waktu":
		newStart = in.Start
		newEnd = in.End
	default:
		return nil, fmt.Errorf("field ubah tidak dikenali: %s", in.Field)
	}

	conflict, err := s.Availability.HasConflict(newCarID, newDriverID, newStart, newEnd, order.ID)
	if err != nil {
		return nil, err
	}
	if conflict {
		return nil, fmt.Errorf("perubahan ini bentrok dengan order aktif lain")
	}

	// If car/driver changed, free up the old ones and mark the new ones busy.
	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if newCarID != order.CarID {
			if err := tx.Model(&models.Car{}).Where("id = ?", order.CarID).
				Update("status", models.CarAvailable).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Car{}).Where("id = ?", newCarID).
				Update("status", models.CarOnTrip).Error; err != nil {
				return err
			}
		}
		if newDriverID != order.DriverID {
			if err := tx.Model(&models.Driver{}).Where("id = ?", order.DriverID).
				Update("status", models.DriverAvailable).Error; err != nil {
				return err
			}
			if err := tx.Model(&models.Driver{}).Where("id = ?", newDriverID).
				Update("status", models.DriverOnTrip).Error; err != nil {
				return err
			}
		}

		return tx.Model(&order).Updates(map[string]any{
			"car_id":          newCarID,
			"driver_id":       newDriverID,
			"pickup_datetime": newStart,
			"return_datetime": newEnd,
			"last_edited_at":  time.Now().In(parser.JakartaLocation),
		}).Error
	})
	if err != nil {
		return nil, err
	}

	if err := s.DB.Preload("Car").Preload("Driver").First(&order, order.ID).Error; err != nil {
		return nil, err
	}
	return &order, nil
}

// CompleteTrip is called when a driver reports a trip finished. It finds
// the driver's current active order, marks it completed, and updates the
// driver's last known location.
func (s *BookingService) CompleteTrip(driverPhone, location string) (*models.Order, error) {
	driver, err := FindDriverByPhone(s.DB, driverPhone)
	if err != nil {
		return nil, fmt.Errorf("nomor ini belum terdaftar sebagai driver")
	}

	var order models.Order
	err = s.DB.Where("driver_id = ? AND status = ?", driver.ID, models.OrderActive).
		Order("pickup_datetime desc").First(&order).Error
	if err != nil {
		return nil, fmt.Errorf("tidak ada order aktif untuk driver ini")
	}

	err = s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&order).Update("status", models.OrderCompleted).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Driver{}).Where("id = ?", driver.ID).
			Updates(map[string]any{
				"status":        models.DriverAvailable,
				"last_location": location,
			}).Error; err != nil {
			return err
		}
		if err := tx.Model(&models.Car{}).Where("id = ?", order.CarID).
			Update("status", models.CarAvailable).Error; err != nil {
			return err
		}
		return tx.Create(&models.LocationLog{
			DriverID: driver.ID,
			Location: location,
		}).Error
	})
	if err != nil {
		return nil, err
	}

	return &order, nil
}

// UpdateDriverPosition updates a driver's location without completing a
// trip — for idle check-ins.
func (s *BookingService) UpdateDriverPosition(driverPhone, location string) error {
	driver, err := FindDriverByPhone(s.DB, driverPhone)
	if err != nil {
		return fmt.Errorf("nomor ini belum terdaftar sebagai driver")
	}

	return s.DB.Transaction(func(tx *gorm.DB) error {
		if err := tx.Model(&models.Driver{}).Where("id = ?", driver.ID).
			Update("last_location", location).Error; err != nil {
			return err
		}
		return tx.Create(&models.LocationLog{
			DriverID: driver.ID,
			Location: location,
		}).Error
	})
}
