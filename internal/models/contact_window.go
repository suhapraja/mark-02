package models

import "time"

// ContactWindow records when a number last sent us a message.
//
// WhatsApp only allows free-form messages within 24 hours of the
// recipient's own last message; outside that, only approved templates
// are delivered. Meta does not report this at send time — the API
// returns 200 and the message is dropped later, surfacing only as a
// 131047 status webhook. Tracking inbound messages ourselves is the
// only way to decide correctly *before* sending.
type ContactWindow struct {
	Phone         string    `gorm:"primaryKey;size:32" json:"phone"`
	LastInboundAt time.Time `gorm:"not null" json:"last_inbound_at"`
}
