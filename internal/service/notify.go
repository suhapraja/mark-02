package service

import (
	"log"
	"time"

	"github.com/suhapraja/mark-02/internal/models"
	"github.com/suhapraja/mark-02/internal/whatsapp"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// Template names and language, as registered with Meta. Sending a
// template that isn't APPROVED fails, so NotifyNewOrder reports whether
// the driver was actually reachable rather than failing silently.
const (
	TemplateNewOrder = "notifikasi_order_baru"
	TemplateLang     = "id"
)

// windowDuration is WhatsApp's 24-hour customer service window, shortened
// by an hour so a message composed near the boundary doesn't get dropped
// between our check and Meta's.
const windowDuration = 23 * time.Hour

type Notifier struct {
	DB *gorm.DB
	WA *whatsapp.Client
}

func NewNotifier(db *gorm.DB, wa *whatsapp.Client) *Notifier {
	return &Notifier{DB: db, WA: wa}
}

// RecordInbound notes that a number just messaged us, opening its
// free-form window.
func (n *Notifier) RecordInbound(phone string) {
	phone = NormalizePhone(phone)
	if phone == "" {
		return
	}
	err := n.DB.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "phone"}},
		DoUpdates: clause.AssignmentColumns([]string{"last_inbound_at"}),
	}).Create(&models.ContactWindow{
		Phone:         phone,
		LastInboundAt: time.Now(),
	}).Error
	if err != nil {
		log.Printf("failed to record inbound window for %s: %v", phone, err)
	}
}

// InWindow reports whether a free-form message to this number will be
// delivered.
func (n *Notifier) InWindow(phone string) bool {
	var cw models.ContactWindow
	if err := n.DB.Where("phone = ?", NormalizePhone(phone)).First(&cw).Error; err != nil {
		return false
	}
	return time.Since(cw.LastInboundAt) < windowDuration
}

// Text sends a free-form message. Use only when InWindow is true;
// outside the window Meta accepts the message and silently drops it.
func (n *Notifier) Text(to, body string) error {
	return n.WA.SendText(to, body)
}

// NotifyNewOrder tells a driver about a new trip, using a free-form
// message when their window is open and the approved template when it
// isn't. Returns false if the driver could not be reached at all, so
// the caller can tell the admin to phone them instead.
func (n *Notifier) NotifyNewOrder(order *models.Order, freeform string, templateParams []string) bool {
	phone := order.Driver.Phone

	if n.InWindow(phone) {
		if err := n.WA.SendText(phone, freeform); err != nil {
			log.Printf("free-form notify failed for driver %s: %v", phone, err)
			return false
		}
		return true
	}

	// Outside the window: only an approved template will be delivered.
	if err := n.WA.SendTemplate(phone, TemplateNewOrder, TemplateLang, templateParams); err != nil {
		log.Printf("template notify failed for driver %s (template %q): %v",
			phone, TemplateNewOrder, err)
		return false
	}
	return true
}

// CanReachFreeform is used for notifications that have no template
// (cancel, edit). Callers should warn the admin when this is false.
func (n *Notifier) CanReachFreeform(phone string) bool {
	return n.InWindow(phone)
}
