package service

import (
	"time"

	"github.com/suhapraja/mark-02/internal/models"
	"gorm.io/gorm"
)

type AvailabilityService struct {
	DB *gorm.DB
}

func NewAvailabilityService(db *gorm.DB) *AvailabilityService {
	return &AvailabilityService{DB: db}
}

// AvailableCars returns cars that are not under maintenance and have no
// active order overlapping the given time range.
func (s *AvailabilityService) AvailableCars(start, end time.Time) ([]models.Car, error) {
	var busyCarIDs []uint
	if err := s.DB.Model(&models.Order{}).
		Where("status = ?", models.OrderActive).
		Where("pickup_datetime < ? AND return_datetime > ?", end, start).
		Pluck("car_id", &busyCarIDs).Error; err != nil {
		return nil, err
	}

	var cars []models.Car
	q := s.DB.Where("status != ?", models.CarMaintenance)
	if len(busyCarIDs) > 0 {
		q = q.Where("id NOT IN ?", busyCarIDs)
	}
	if err := q.Order("plate_number").Find(&cars).Error; err != nil {
		return nil, err
	}
	return cars, nil
}

// AvailableDrivers returns drivers with no active order overlapping the
// given time range, including each driver's last known location.
func (s *AvailabilityService) AvailableDrivers(start, end time.Time) ([]models.Driver, error) {
	var busyDriverIDs []uint
	if err := s.DB.Model(&models.Order{}).
		Where("status = ?", models.OrderActive).
		Where("pickup_datetime < ? AND return_datetime > ?", end, start).
		Pluck("driver_id", &busyDriverIDs).Error; err != nil {
		return nil, err
	}

	var drivers []models.Driver
	q := s.DB
	if len(busyDriverIDs) > 0 {
		q = q.Where("id NOT IN ?", busyDriverIDs)
	}
	if err := q.Order("name").Find(&drivers).Error; err != nil {
		return nil, err
	}
	return drivers, nil
}

// HasConflict checks whether a given car or driver already has an active
// order overlapping the requested range. excludeOrderID is used when
// editing an existing order, so it doesn't conflict with itself.
func (s *AvailabilityService) HasConflict(carID, driverID uint, start, end time.Time, excludeOrderID uint) (bool, error) {
	var count int64
	q := s.DB.Model(&models.Order{}).
		Where("status = ?", models.OrderActive).
		Where("(car_id = ? OR driver_id = ?)", carID, driverID).
		Where("pickup_datetime < ? AND return_datetime > ?", end, start)

	if excludeOrderID != 0 {
		q = q.Where("id != ?", excludeOrderID)
	}

	if err := q.Count(&count).Error; err != nil {
		return false, err
	}
	return count > 0, nil
}
