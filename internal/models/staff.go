package models

import "time"

type StaffRole string

const (
	// RoleSuperadmin can do everything an admin can, plus manage the
	// staff/driver registry and change car maintenance status.
	RoleSuperadmin StaffRole = "superadmin"
	// RoleAdmin handles day-to-day work: availability, bookings, export.
	RoleAdmin StaffRole = "admin"
)

// Staff is anyone who talks to the bot in a management capacity — the
// business owner and the developer as superadmins, plus regular admins.
// Drivers are tracked separately in Driver, since they have their own
// operational state (location, on-trip status).
type Staff struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"not null" json:"name"`
	Phone     string    `gorm:"uniqueIndex;not null" json:"phone"` // WhatsApp number, e.g. 62812xxxxxxx
	Role      StaffRole `gorm:"type:varchar(20);not null;default:admin" json:"role"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TableName overrides GORM's default pluralization, which would
// otherwise produce "staffs".
func (Staff) TableName() string {
	return "staff"
}

func (s Staff) IsSuperadmin() bool {
	return s.Role == RoleSuperadmin
}
