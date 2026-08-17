package handlers

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strings"

	"github.com/suhapraja/mark-02/internal/models"
	"github.com/suhapraja/mark-02/internal/parser"
	"github.com/suhapraja/mark-02/internal/service"
	"github.com/suhapraja/mark-02/internal/whatsapp"
)

type WebhookHandler struct {
	VerifyToken  string
	WA           *whatsapp.Client
	Availability *service.AvailabilityService
	Booking      *service.BookingService
	Export       *service.ExportService
	Staff        *service.StaffService
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
			// Delivery receipts for messages we sent. Logged rather than
			// acted on — a "failed" status here is the only place Meta
			// reports that an accepted message never reached the user.
			for _, st := range change.Value.Statuses {
				if len(st.Errors) > 0 {
					for _, e := range st.Errors {
						log.Printf("message %s to %s FAILED: code=%d title=%q message=%q details=%q",
							st.ID, st.RecipientID, e.Code, e.Title, e.Message, e.ErrorData.Details)
					}
					continue
				}
				log.Printf("message %s to %s status=%s", st.ID, st.RecipientID, st.Status)
			}

			for _, msg := range change.Value.Messages {
				log.Printf("incoming message from %s type=%s", msg.From, msg.Type)
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
	// Who is speaking decides how the message is read. Staff and drivers
	// are mutually exclusive, enforced when either is registered.
	if staff, err := service.FindStaffByPhone(h.Booking.DB, from); err == nil {
		h.handleStaff(from, *staff, text)
		return
	}

	if _, err := service.FindDriverByPhone(h.Booking.DB, from); err == nil {
		h.handleDriver(from, text)
		return
	}

	h.reply(from, "Nomor ini belum terdaftar di sistem. Hubungi admin jika ini seharusnya terdaftar.")
}

func (h *WebhookHandler) handleStaff(from string, staff models.Staff, text string) {
	cmd, err := parser.ParseAdminCommand(text)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}

	if cmd.Type.SuperadminOnly() && !staff.IsSuperadmin() {
		h.reply(from, "⚠️ Perintah ini hanya untuk superadmin.")
		return
	}

	switch cmd.Type {
	case parser.CmdHelp:
		h.reply(from, helpTextFor(staff))

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

	case parser.CmdAddStaff:
		h.handleAddStaff(from, cmd)

	case parser.CmdAddDriver:
		h.handleAddDriver(from, cmd)

	case parser.CmdRemoveStaff:
		h.handleRemoveStaff(from, cmd)

	case parser.CmdRemoveDriver:
		h.handleRemoveDriver(from, cmd)

	case parser.CmdAddCar:
		h.handleAddCar(from, cmd)

	case parser.CmdRemoveCar:
		h.handleRemoveCar(from, cmd)

	case parser.CmdListStaff:
		h.handleListStaff(from)

	case parser.CmdListCars:
		h.handleListCars(from)

	case parser.CmdListDrivers:
		h.handleListDrivers(from)

	case parser.CmdSetMaintenance:
		h.handleSetCarStatus(from, cmd, models.CarMaintenance)

	case parser.CmdSetReady:
		h.handleSetCarStatus(from, cmd, models.CarAvailable)

	default:
		h.reply(from, "Perintah tidak dikenali. Ketik \"help\" untuk melihat daftar perintah.")
	}
}

func (h *WebhookHandler) handleAddStaff(from string, cmd parser.Command) {
	staff, err := h.Staff.AddStaff(cmd.PersonName, cmd.PersonPhone, models.RoleAdmin)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	h.reply(from, fmt.Sprintf("✅ Admin ditambahkan: %s (%s)", staff.Name, staff.Phone))
	h.reply(staff.Phone, fmt.Sprintf(
		"👋 Halo %s, kamu sekarang terdaftar sebagai admin di sistem rental.\nKetik \"help\" untuk melihat daftar perintah.",
		staff.Name))
}

func (h *WebhookHandler) handleAddDriver(from string, cmd parser.Command) {
	driver, err := h.Staff.AddDriver(cmd.PersonName, cmd.PersonPhone)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	h.reply(from, fmt.Sprintf("✅ Driver ditambahkan: %s (%s)", driver.Name, driver.Phone))
	h.reply(driver.Phone, fmt.Sprintf(
		"👋 Halo %s, kamu sekarang terdaftar sebagai driver.\nSetelah trip selesai, balas: \"selesai, sekarang di [kota]\"",
		driver.Name))
}

func (h *WebhookHandler) handleRemoveStaff(from string, cmd parser.Command) {
	staff, err := h.Staff.RemoveStaff(cmd.PersonPhone)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	h.reply(from, fmt.Sprintf("🗑️ Admin dihapus: %s (%s)", staff.Name, staff.Phone))
}

func (h *WebhookHandler) handleRemoveDriver(from string, cmd parser.Command) {
	driver, err := h.Staff.RemoveDriver(cmd.PersonPhone)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	h.reply(from, fmt.Sprintf("🗑️ Driver dihapus: %s (%s)", driver.Name, driver.Phone))
}

func (h *WebhookHandler) handleAddCar(from string, cmd parser.Command) {
	car, err := h.Staff.AddCar(cmd.CarPlate, cmd.CarModel)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	h.reply(from, fmt.Sprintf("✅ Mobil ditambahkan: %s (%s)", car.PlateNumber, car.Model))
}

func (h *WebhookHandler) handleRemoveCar(from string, cmd parser.Command) {
	car, err := h.Staff.RemoveCar(cmd.CarQuery)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	h.reply(from, fmt.Sprintf("🗑️ Mobil dihapus: %s (%s)", car.PlateNumber, car.Model))
}

func (h *WebhookHandler) handleListCars(from string) {
	cars, err := h.Staff.ListCars()
	if err != nil {
		h.reply(from, "⚠️ Gagal mengambil daftar mobil: "+err.Error())
		return
	}

	msg := "🚗 Daftar mobil:\n"
	if len(cars) == 0 {
		msg += "- Belum ada mobil terdaftar\n"
	}
	for _, c := range cars {
		msg += fmt.Sprintf("- %s (%s) — %s\n", c.PlateNumber, c.Model, c.Status)
	}
	h.reply(from, msg)
}

func (h *WebhookHandler) handleListDrivers(from string) {
	drivers, err := h.Staff.ListDrivers()
	if err != nil {
		h.reply(from, "⚠️ Gagal mengambil daftar driver: "+err.Error())
		return
	}

	msg := "👤 Daftar driver:\n"
	if len(drivers) == 0 {
		msg += "- Belum ada driver terdaftar\n"
	}
	for _, d := range drivers {
		loc := d.LastLocation
		if loc == "" {
			loc = "lokasi tidak diketahui"
		}
		msg += fmt.Sprintf("- %s (%s) — %s, terakhir di %s\n", d.Name, d.Phone, d.Status, loc)
	}
	h.reply(from, msg)
}

func (h *WebhookHandler) handleListStaff(from string) {
	staff, err := h.Staff.ListStaff()
	if err != nil {
		h.reply(from, "⚠️ Gagal mengambil daftar staff: "+err.Error())
		return
	}

	msg := "👥 Daftar staff:\n"
	if len(staff) == 0 {
		msg += "- Belum ada staff terdaftar\n"
	}
	for _, s := range staff {
		msg += fmt.Sprintf("- %s (%s) — %s\n", s.Name, s.Phone, s.Role)
	}
	h.reply(from, msg)
}

func (h *WebhookHandler) handleSetCarStatus(from string, cmd parser.Command, status models.CarStatus) {
	car, err := h.Staff.SetCarStatus(cmd.CarQuery, status)
	if err != nil {
		h.reply(from, "⚠️ "+err.Error())
		return
	}
	if status == models.CarMaintenance {
		h.reply(from, fmt.Sprintf("🔧 %s (%s) ditandai sedang perbaikan", car.PlateNumber, car.Model))
		return
	}
	h.reply(from, fmt.Sprintf("✅ %s (%s) siap dipakai lagi", car.PlateNumber, car.Model))
}

// notifyStaff sends an operational update to everyone on the staff list.
func (h *WebhookHandler) notifyStaff(text string) {
	phones, err := h.Staff.StaffPhones()
	if err != nil {
		log.Printf("failed to load staff phones: %v", err)
		return
	}
	for _, p := range phones {
		h.reply(p, text)
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
		h.notifyStaff(fmt.Sprintf("ℹ️ Order #%d selesai, driver kini di %s", order.ID, cmd.Location))

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
		CarQuery:      cmd.CarQuery,
		DriverQuery:   cmd.DriverQuery,
		CustomerName:  cmd.CustomerName,
		CustomerPhone: cmd.CustomerPhone,
		Destination:   cmd.Destination,
		Pemesan:       cmd.Pemesan,
		PickupPoint:   cmd.PickupPoint,
		Partner:       cmd.Partner,
		Notes:         cmd.Notes,
		Start:         cmd.RangeStart,
		End:           cmd.RangeEnd,
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
	// Logged so replies are visible even when the send itself fails
	// (e.g. outside WhatsApp's 24-hour window), which is otherwise
	// impossible to diagnose from the outside.
	log.Printf("reply to %s: %s", to, strings.ReplaceAll(text, "\n", " | "))
	if err := h.WA.SendText(to, text); err != nil {
		log.Printf("failed to send message to %s: %v", to, err)
	}
}

// helpTextFor returns the command list appropriate to the sender's role,
// so admins aren't shown commands they can't run.
func helpTextFor(staff models.Staff) string {
	if staff.IsSuperadmin() {
		return adminHelpText + "\n\n" + superadminHelpText
	}
	return adminHelpText
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

const superadminHelpText = `👑 Khusus superadmin:

tambah admin [nama], [nomor]
  cth: tambah admin Budi, 08123456789

tambah driver [nama], [nomor]
  cth: tambah driver Andi, 08123456790

tambah mobil [plat], [model]
  cth: tambah mobil B 1234 XYZ, Toyota Avanza

hapus admin [nomor]
hapus driver [nomor]
hapus mobil [plat]

daftar staff
daftar driver
daftar mobil

maintenance [mobil]
  cth: maintenance Avanza B1234

siap [mobil]
  Tandai mobil selesai perbaikan`
