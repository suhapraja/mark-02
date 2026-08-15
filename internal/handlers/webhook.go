package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"

	"github.com/suhapraja/mark-02/internal/parser"
	"github.com/suhapraja/mark-02/internal/service"
	"github.com/suhapraja/mark-02/internal/whatsapp"
)

type WebhookHandler struct {
	VerifyToken  string
	AdminPhone   string
	WA           *whatsapp.Client
	Availability *service.AvailabilityService
	Booking      *service.BookingService
	Export       *service.ExportService
}

// VerifyWebhook handles Meta's GET verification challenge when you first
// configure the webhook URL in the Meta App dashboard.
func (h *WebhookHandler) VerifyWebhook(w http.ResponseWriter, r *http.Request) {
	mode := r.URL.Query().Get("hub.mode")
	token := r.URL.Query().Get("hub.verify_token")
	challenge := r.URL.Query().Get("hub.challenge")

	if mode == "subscribe" && token == h.VerifyToken {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(challenge))
		return
	}
	w.WriteHeader(http.StatusForbidden)
}

// ReceiveMessage handles incoming WhatsApp messages (POST from Meta).
func (h *WebhookHandler) ReceiveMessage(w http.ResponseWriter, r *http.Request) {
	var payload whatsapp.WebhookPayload
	if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
		log.Printf("failed to decode webhook payload: %v", err)
		w.WriteHeader(http.StatusOK) // Meta expects 200 regardless, to stop retries
		return
	}

	for _, entry := range payload.Entry {
		for _, change := range entry.Changes {
			for _, msg := range change.Value.Messages {
				if msg.Type != "text" {
					h.reply(msg.From, "Maaf, saat ini bot hanya bisa membaca pesan teks.")
					continue
				}
				h.handleText(msg.From, msg.Text.Body)
			}
		}
	}

	w.WriteHeader(http.StatusOK)
}

func (h *WebhookHandler) handleText(from, text string) {
	if from == h.AdminPhone {
		h.handleAdmin(from, text)
		return
	}

	if _, err := service.FindDriverByPhone(h.Booking.DB, from); err == nil {
		h.handleDriver(from, text)
		return
	}

	h.reply(from, "Nomor ini belum terdaftar di sistem. Hubungi admin jika ini seharusnya terdaftar.")
}

func (h *WebhookHandler) handleAdmin(from, text string) {
	cmd, err := parser.ParseAdminCommand(text)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}

	switch cmd.Type {
	case parser.CmdHelp:
		h.reply(from, adminHelpText)

	case parser.CmdCheckAvailability:
		h.handleCheckAvailability(from, cmd)

	case parser.CmdBooking:
		h.handleBooking(from, cmd)

	case parser.CmdCancel:
		h.handleCancel(from, cmd)

	case parser.CmdEdit:
		h.handleEdit(from, cmd)

	case parser.CmdExport:
		h.handleExport(from)

	default:
		h.reply(from, "Perintah tidak dikenali. Ketik \"help\" untuk melihat daftar perintah.")
	}
}

func (h *WebhookHandler) handleDriver(from, text string) {
	cmd, err := parser.ParseDriverCommand(text)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}

	switch cmd.Type {
	case parser.CmdDriverComplete:
		order, err := h.Booking.CompleteTrip(from, cmd.Location)
		if err != nil {
			h.reply(from, "⚠️ "+err.Error())
			return
		}
		h.reply(from, fmt.Sprintf("✅ Trip order #%d selesai. Lokasi tercatat: %s", order.ID, cmd.Location))
		h.reply(h.AdminPhone, fmt.Sprintf("ℹ️ Order #%d selesai, driver kini di %s", order.ID, cmd.Location))

	case parser.CmdDriverPosition:
		if err := h.Booking.UpdateDriverPosition(from, cmd.Location); err != nil {
			h.reply(from, "⚠️ "+err.Error())
			return
		}
		h.reply(from, "📍 Lokasi diperbarui: "+cmd.Location)

	default:
		h.reply(from, `Perintah tidak dikenali. Gunakan "selesai, sekarang di [kota]" atau "posisi [kota]".`)
	}
}

func (h *WebhookHandler) handleCheckAvailability(from string, cmd parser.Command) {
	cars, err := h.Availability.AvailableCars(cmd.RangeStart, cmd.RangeEnd)
	if err != nil {
		h.reply(from, "⚠️ Gagal mengecek mobil: "+err.Error())
		return
	}
	drivers, err := h.Availability.AvailableDrivers(cmd.RangeStart, cmd.RangeEnd)
	if err != nil {
		h.reply(from, "⚠️ Gagal mengecek driver: "+err.Error())
		return
	}

	msg := fmt.Sprintf("🚗 Mobil tersedia (%s - %s):\n",
		cmd.RangeStart.Format("2 Jan 15:04"), cmd.RangeEnd.Format("2 Jan 15:04"))
	if len(cars) == 0 {
		msg += "- Tidak ada mobil tersedia\n"
	}
	for _, c := range cars {
		msg += fmt.Sprintf("- %s (%s)\n", c.PlateNumber, c.Model)
	}

	msg += "\n👤 Driver tersedia:\n"
	if len(drivers) == 0 {
		msg += "- Tidak ada driver tersedia\n"
	}
	for _, d := range drivers {
		loc := d.LastLocation
		if loc == "" {
			loc = "lokasi tidak diketahui"
		}
		msg += fmt.Sprintf("- %s (terakhir di %s)\n", d.Name, loc)
	}

	h.reply(from, msg)
}

func (h *WebhookHandler) handleBooking(from string, cmd parser.Command) {
	order, err := h.Booking.CreateBooking(service.CreateBookingInput{
		CarQuery:     cmd.CarQuery,
		DriverQuery:  cmd.DriverQuery,
		CustomerName: cmd.CustomerName,
		Destination:  cmd.Destination,
		Start:        cmd.RangeStart,
		End:          cmd.RangeEnd,
	})
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}

	h.reply(from, fmt.Sprintf(
		"✅ Order #%d dibuat\nMobil: %s\nDriver: %s\nJemput: %s\nKembali: %s\nCustomer: %s\nTujuan: %s",
		order.ID, order.Car.PlateNumber, order.Driver.Name,
		order.PickupDatetime.In(parser.JakartaLocation).Format("2 Jan 15:04"), order.ReturnDatetime.In(parser.JakartaLocation).Format("2 Jan 15:04"),
		order.CustomerName, order.DestinationCity,
	))

	// Auto-notify the assigned driver.
	h.reply(order.Driver.Phone, fmt.Sprintf(
		"📋 Order baru untuk kamu\nOrder #%d\nMobil: %s\nJemput: %s\nKembali: %s\nCustomer: %s\nTujuan: %s\n\nBalas \"selesai, sekarang di [kota]\" setelah trip selesai.",
		order.ID, order.Car.PlateNumber,
		order.PickupDatetime.In(parser.JakartaLocation).Format("2 Jan 15:04"), order.ReturnDatetime.In(parser.JakartaLocation).Format("2 Jan 15:04"),
		order.CustomerName, order.DestinationCity,
	))
}

func (h *WebhookHandler) handleCancel(from string, cmd parser.Command) {
	order, err := h.Booking.CancelBooking(cmd.OrderID)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	h.reply(from, fmt.Sprintf("🗑️ Order #%d dibatalkan", order.ID))
	h.reply(order.Driver.Phone, fmt.Sprintf("🗑️ Order #%d dibatalkan oleh admin", order.ID))
}

func (h *WebhookHandler) handleEdit(from string, cmd parser.Command) {
	order, err := h.Booking.EditBooking(service.EditBookingInput{
		OrderID: cmd.OrderID,
		Field:   cmd.EditField,
		Value:   cmd.EditValue,
		Start:   cmd.RangeStart,
		End:     cmd.RangeEnd,
	})
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	h.reply(from, fmt.Sprintf(
		"✏️ Order #%d diperbarui\nMobil: %s\nDriver: %s\nJemput: %s\nKembali: %s",
		order.ID, order.Car.PlateNumber, order.Driver.Name,
		order.PickupDatetime.In(parser.JakartaLocation).Format("2 Jan 15:04"), order.ReturnDatetime.In(parser.JakartaLocation).Format("2 Jan 15:04"),
	))
	h.reply(order.Driver.Phone, fmt.Sprintf("✏️ Order #%d kamu telah diperbarui, cek detail terbaru dengan admin.", order.ID))
}

func (h *WebhookHandler) handleExport(from string) {
	data, err := h.Export.GenerateOrdersExcel()
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}

	mediaID, err := h.WA.UploadMedia("orders.xlsx", data,
		"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet")
	if err != nil {
		h.reply(from, "⚠️ Gagal upload file excel: "+err.Error())
		return
	}

	if err := h.WA.SendDocument(from, mediaID, "orders.xlsx"); err != nil {
		h.reply(from, "⚠️ Gagal mengirim file excel: "+err.Error())
	}
}

func (h *WebhookHandler) reply(to, text string) {
	if err := h.WA.SendText(to, text); err != nil {
		log.Printf("failed to send message to %s: %v", to, err)
	}
}

const adminHelpText = `📖 Daftar perintah:

cek mobil [tgl jam] - [tgl jam]
  cth: cek mobil 20 Agustus 08:00 - 22 Agustus 17:00

booking [mobil], [driver], [tgl jam - tgl jam], customer [nama], tujuan [kota]
  cth: booking Avanza B1234, Budi, 20 Agustus 08:00 - 22 Agustus 17:00, customer Yusuf, tujuan Bandung

batal [nomor order]
  cth: batal 12

ubah [nomor order], driver [nama baru]
ubah [nomor order], mobil [mobil baru]
ubah [nomor order], waktu [tgl jam - tgl jam]

export
  Kirim file excel semua data order`
