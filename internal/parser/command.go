package parser

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

type CommandType string

const (
	CmdCheckAvailability CommandType = "check_availability"
	CmdBooking           CommandType = "booking"
	CmdCancel            CommandType = "cancel"
	CmdEdit              CommandType = "edit"
	CmdExport            CommandType = "export"
	CmdDriverComplete    CommandType = "driver_complete"
	CmdDriverPosition    CommandType = "driver_position"
	CmdUnknown           CommandType = "unknown"
	CmdHelp              CommandType = "help"
)

type Command struct {
	Type CommandType

	// check_availability
	RangeStart time.Time
	RangeEnd   time.Time

	// booking
	CarQuery      string
	DriverQuery   string
	CustomerName  string
	Destination   string

	// cancel / edit
	OrderID   uint
	EditField string // "driver" | "mobil" | "waktu"
	EditValue string

	// driver_complete / driver_position
	Location string
}

// ParseAdminCommand interprets a message from the admin (dad) number.
func ParseAdminCommand(raw string) (Command, error) {
	text := strings.TrimSpace(raw)
	lower := strings.ToLower(text)

	switch {
	case lower == "help" || lower == "bantuan":
		return Command{Type: CmdHelp}, nil

	case lower == "export":
		return Command{Type: CmdExport}, nil

	case strings.HasPrefix(lower, "cek mobil") || strings.HasPrefix(lower, "cek"):
		rangeText := stripPrefixCI(text, "cek mobil")
		if rangeText == text {
			rangeText = stripPrefixCI(text, "cek")
		}
		start, end, err := ParseDateTimeRange(rangeText)
		if err != nil {
			return Command{}, err
		}
		return Command{Type: CmdCheckAvailability, RangeStart: start, RangeEnd: end}, nil

	case strings.HasPrefix(lower, "booking"):
		return parseBooking(text)

	case strings.HasPrefix(lower, "batal"):
		idText := strings.TrimSpace(stripPrefixCI(text, "batal"))
		id, err := parseOrderID(idText)
		if err != nil {
			return Command{}, err
		}
		return Command{Type: CmdCancel, OrderID: id}, nil

	case strings.HasPrefix(lower, "ubah"):
		return parseEdit(text)

	default:
		return Command{Type: CmdUnknown}, nil
	}
}

// ParseDriverCommand interprets a message from a registered driver number.
func ParseDriverCommand(raw string) (Command, error) {
	text := strings.TrimSpace(raw)
	lower := strings.ToLower(text)

	switch {
	case strings.HasPrefix(lower, "selesai"):
		// e.g. "selesai, sekarang di Bandung"
		idx := strings.Index(lower, "di ")
		if idx == -1 {
			return Command{}, fmt.Errorf(`format tidak dikenali, gunakan contoh: "selesai, sekarang di Bandung"`)
		}
		location := strings.TrimSpace(text[idx+3:])
		if location == "" {
			return Command{}, fmt.Errorf("lokasi tidak boleh kosong")
		}
		return Command{Type: CmdDriverComplete, Location: location}, nil

	case strings.HasPrefix(lower, "posisi"):
		location := strings.TrimSpace(stripPrefixCI(text, "posisi"))
		if location == "" {
			return Command{}, fmt.Errorf("lokasi tidak boleh kosong")
		}
		return Command{Type: CmdDriverPosition, Location: location}, nil

	default:
		return Command{Type: CmdUnknown}, nil
	}
}

func parseBooking(text string) (Command, error) {
	body := strings.TrimSpace(stripPrefixCI(text, "booking"))
	parts := splitAndTrim(body, ",")

	// Expected: [car, driver, datetime range, "customer X", "tujuan Y" (optional)]
	if len(parts) < 4 {
		return Command{}, fmt.Errorf(
			"format booking tidak lengkap, gunakan contoh:\n" +
				"booking Avanza B1234, Budi, 20 Agustus 08:00 - 22 Agustus 17:00, customer Yusuf, tujuan Bandung")
	}

	carQuery := parts[0]
	driverQuery := parts[1]

	start, end, err := ParseDateTimeRange(parts[2])
	if err != nil {
		return Command{}, err
	}

	customerName := stripPrefixCI(parts[3], "customer")
	customerName = strings.TrimSpace(customerName)
	if customerName == "" {
		return Command{}, fmt.Errorf("nama customer tidak boleh kosong")
	}

	destination := ""
	if len(parts) >= 5 {
		destination = strings.TrimSpace(stripPrefixCI(parts[4], "tujuan"))
	}

	return Command{
		Type:         CmdBooking,
		CarQuery:     carQuery,
		DriverQuery:  driverQuery,
		RangeStart:   start,
		RangeEnd:     end,
		CustomerName: customerName,
		Destination:  destination,
	}, nil
}

func parseEdit(text string) (Command, error) {
	body := strings.TrimSpace(stripPrefixCI(text, "ubah"))
	parts := splitAndTrim(body, ",")

	if len(parts) < 2 {
		return Command{}, fmt.Errorf(
			"format ubah tidak lengkap, gunakan contoh:\n" +
				"ubah 12, driver Budi\n" +
				"ubah 12, mobil Avanza B1234\n" +
				"ubah 12, waktu 20 Agustus 08:00 - 22 Agustus 17:00")
	}

	id, err := parseOrderID(parts[0])
	if err != nil {
		return Command{}, err
	}

	fieldValue := parts[1]
	lowerFV := strings.ToLower(fieldValue)

	cmd := Command{Type: CmdEdit, OrderID: id}

	switch {
	case strings.HasPrefix(lowerFV, "driver"):
		cmd.EditField = "driver"
		cmd.EditValue = strings.TrimSpace(stripPrefixCI(fieldValue, "driver"))
	case strings.HasPrefix(lowerFV, "mobil"):
		cmd.EditField = "mobil"
		cmd.EditValue = strings.TrimSpace(stripPrefixCI(fieldValue, "mobil"))
	case strings.HasPrefix(lowerFV, "waktu"):
		rangeText := strings.TrimSpace(stripPrefixCI(fieldValue, "waktu"))
		start, end, err := ParseDateTimeRange(rangeText)
		if err != nil {
			return Command{}, err
		}
		cmd.EditField = "waktu"
		cmd.RangeStart = start
		cmd.RangeEnd = end
	default:
		return Command{}, fmt.Errorf(`jenis perubahan tidak dikenali, gunakan salah satu: "driver", "mobil", atau "waktu"`)
	}

	return cmd, nil
}

func parseOrderID(text string) (uint, error) {
	text = strings.TrimSpace(text)
	// Only take the leading numeric token, in case there's trailing text.
	fields := strings.Fields(text)
	if len(fields) == 0 {
		return 0, fmt.Errorf("nomor order tidak ditemukan")
	}
	id, err := strconv.ParseUint(fields[0], 10, 64)
	if err != nil {
		return 0, fmt.Errorf("nomor order tidak valid: %s", fields[0])
	}
	return uint(id), nil
}

func stripPrefixCI(s, prefix string) string {
	if len(s) < len(prefix) {
		return s
	}
	if strings.EqualFold(s[:len(prefix)], prefix) {
		return strings.TrimSpace(s[len(prefix):])
	}
	return s
}

func splitAndTrim(s, sep string) []string {
	raw := strings.Split(s, sep)
	out := make([]string, 0, len(raw))
	for _, r := range raw {
		trimmed := strings.TrimSpace(r)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
