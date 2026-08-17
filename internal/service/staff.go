package service

import (
	"fmt"
	"strings"

	"github.com/suhapraja/mark-02/internal/models"
	"gorm.io/gorm"
)

type StaffService struct {
	DB *gorm.DB
}

func NewStaffService(db *gorm.DB) *StaffService {
	return &StaffService{DB: db}
}

// NormalizePhone converts the various ways someone might type an
// Indonesian number into the format WhatsApp uses (62 + number, digits
// only): "+62 812-3456-7890", "0812 3456 7890" and "62812345678" all
// normalize to the same string.
func NormalizePhone(raw string) string {
	var digits strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			digits.WriteRune(r)
		}
	}
	p := digits.String()

	switch {
	case strings.HasPrefix(p, "62"):
		return p
	case strings.HasPrefix(p, "0"):
		return "62" + strings.TrimPrefix(p, "0")
	case p == "":
		return ""
	default:
		// Bare local number without leading 0, e.g. "812345678".
		return "62" + p
	}
}

// FindStaffByPhone looks up a staff member by WhatsApp number.
func FindStaffByPhone(db *gorm.DB, phone string) (*models.Staff, error) {
	var staff models.Staff
	if err := db.Where("phone = ?", NormalizePhone(phone)).First(&staff).Error; err != nil {
		return nil, err
	}
	return &staff, nil
}

// BootstrapSuperadmins makes sure every phone listed in SUPERADMIN_PHONES
// exists as a superadmin. Runs on startup so there is always a way back
// in if the staff table is emptied or a role is changed by mistake.
func (s *StaffService) BootstrapSuperadmins(phones []string) error {
	for _, raw := range phones {
		phone := NormalizePhone(raw)
		if phone == "" {
			continue
		}

		var staff models.Staff
		err := s.DB.Where("phone = ?", phone).First(&staff).Error
		switch {
		case err == gorm.ErrRecordNotFound:
			staff = models.Staff{Name: "Superadmin", Phone: phone, Role: models.RoleSuperadmin}
			if err := s.DB.Create(&staff).Error; err != nil {
				return fmt.Errorf("bootstrap superadmin %s: %w", phone, err)
			}
		case err != nil:
			return fmt.Errorf("bootstrap superadmin %s: %w", phone, err)
		case staff.Role != models.RoleSuperadmin:
			if err := s.DB.Model(&staff).Update("role", models.RoleSuperadmin).Error; err != nil {
				return fmt.Errorf("bootstrap superadmin %s: %w", phone, err)
			}
		}
	}
	return nil
}

// AddStaff registers a new admin (or superadmin). Existing numbers have
// their name/role updated rather than erroring, so re-adding someone to
// change their role works the way you'd expect.
func (s *StaffService) AddStaff(name, rawPhone string, role models.StaffRole) (*models.Staff, error) {
	phone := NormalizePhone(rawPhone)
	if phone == "" {
		return nil, fmt.Errorf("nomor tidak valid")
	}
	if name == "" {
		return nil, fmt.Errorf("nama tidak boleh kosong")
	}

	// A number can't be both staff and driver — the bot decides how to
	// read a message purely from who sent it.
	var driverCount int64
	if err := s.DB.Model(&models.Driver{}).Where("phone = ?", phone).Count(&driverCount).Error; err != nil {
		return nil, err
	}
	if driverCount > 0 {
		return nil, fmt.Errorf("nomor %s sudah terdaftar sebagai driver, hapus dulu dari daftar driver", phone)
	}

	var staff models.Staff
	err := s.DB.Where("phone = ?", phone).First(&staff).Error
	if err == gorm.ErrRecordNotFound {
		staff = models.Staff{Name: name, Phone: phone, Role: role}
		if err := s.DB.Create(&staff).Error; err != nil {
			return nil, err
		}
		return &staff, nil
	}
	if err != nil {
		return nil, err
	}

	if err := s.DB.Model(&staff).Updates(map[string]any{"name": name, "role": role}).Error; err != nil {
		return nil, err
	}
	return &staff, nil
}

// RemoveStaff deletes a staff member by phone number. The last remaining
// superadmin cannot be removed, so the system can't be locked out.
func (s *StaffService) RemoveStaff(rawPhone string) (*models.Staff, error) {
	phone := NormalizePhone(rawPhone)

	var staff models.Staff
	if err := s.DB.Where("phone = ?", phone).First(&staff).Error; err != nil {
		return nil, fmt.Errorf("staff dengan nomor %s tidak ditemukan", phone)
	}

	if staff.Role == models.RoleSuperadmin {
		var count int64
		if err := s.DB.Model(&models.Staff{}).
			Where("role = ?", models.RoleSuperadmin).Count(&count).Error; err != nil {
			return nil, err
		}
		if count <= 1 {
			return nil, fmt.Errorf("tidak bisa menghapus superadmin terakhir")
		}
	}

	if err := s.DB.Delete(&staff).Error; err != nil {
		return nil, err
	}
	return &staff, nil
}

// ListStaff returns all staff, superadmins first.
func (s *StaffService) ListStaff() ([]models.Staff, error) {
	var staff []models.Staff
	err := s.DB.Order("role, name").Find(&staff).Error
	return staff, err
}

// AddDriver registers a new driver.
func (s *StaffService) AddDriver(name, rawPhone string) (*models.Driver, error) {
	phone := NormalizePhone(rawPhone)
	if phone == "" {
		return nil, fmt.Errorf("nomor tidak valid")
	}
	if name == "" {
		return nil, fmt.Errorf("nama tidak boleh kosong")
	}

	var staffCount int64
	if err := s.DB.Model(&models.Staff{}).Where("phone = ?", phone).Count(&staffCount).Error; err != nil {
		return nil, err
	}
	if staffCount > 0 {
		return nil, fmt.Errorf("nomor %s sudah terdaftar sebagai admin, hapus dulu dari daftar staff", phone)
	}

	var driver models.Driver
	err := s.DB.Where("phone = ?", phone).First(&driver).Error
	if err == gorm.ErrRecordNotFound {
		driver = models.Driver{Name: name, Phone: phone, Status: models.DriverAvailable}
		if err := s.DB.Create(&driver).Error; err != nil {
			return nil, err
		}
		return &driver, nil
	}
	if err != nil {
		return nil, err
	}

	if err := s.DB.Model(&driver).Update("name", name).Error; err != nil {
		return nil, err
	}
	return &driver, nil
}

// RemoveDriver deletes a driver, refusing while they still have an
// active order so an in-progress trip can't lose its driver.
func (s *StaffService) RemoveDriver(rawPhone string) (*models.Driver, error) {
	phone := NormalizePhone(rawPhone)

	var driver models.Driver
	if err := s.DB.Where("phone = ?", phone).First(&driver).Error; err != nil {
		return nil, fmt.Errorf("driver dengan nomor %s tidak ditemukan", phone)
	}

	var active int64
	if err := s.DB.Model(&models.Order{}).
		Where("driver_id = ? AND status = ?", driver.ID, models.OrderActive).
		Count(&active).Error; err != nil {
		return nil, err
	}
	if active > 0 {
		return nil, fmt.Errorf("driver %s masih punya order aktif, selesaikan atau batalkan dulu", driver.Name)
	}

	if err := s.DB.Delete(&driver).Error; err != nil {
		return nil, err
	}
	return &driver, nil
}

// SetCarStatus marks a car as under maintenance or back in service.
func (s *StaffService) SetCarStatus(carQuery string, status models.CarStatus) (*models.Car, error) {
	car, err := FindCarByQuery(s.DB, carQuery)
	if err != nil {
		return nil, err
	}

	if status == models.CarMaintenance {
		var active int64
		if err := s.DB.Model(&models.Order{}).
			Where("car_id = ? AND status = ?", car.ID, models.OrderActive).
			Count(&active).Error; err != nil {
			return nil, err
		}
		if active > 0 {
			return nil, fmt.Errorf("mobil %s masih punya order aktif, batalkan dulu ordernya", car.PlateNumber)
		}
	}

	if err := s.DB.Model(car).Update("status", status).Error; err != nil {
		return nil, err
	}
	car.Status = status
	return car, nil
}

// StaffPhones returns the numbers to notify about operational events
// (e.g. a driver finishing a trip).
func (s *StaffService) StaffPhones() ([]string, error) {
	var phones []string
	err := s.DB.Model(&models.Staff{}).Order("role").Pluck("phone", &phones).Error
	return phones, err
}
