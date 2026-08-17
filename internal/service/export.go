package service

import (
	"fmt"
	"sort"
	"time"

	"github.com/suhapraja/mark-02/internal/models"
	"github.com/suhapraja/mark-02/internal/parser"
	"github.com/xuri/excelize/v2"
	"gorm.io/gorm"
)

type ExportService struct {
	DB *gorm.DB
}

func NewExportService(db *gorm.DB) *ExportService {
	return &ExportService{DB: db}
}

// Indonesian month names, used for sheet titles ("Jan'26") matching the
// spreadsheet this export replaces.
var monthShort = map[time.Month]string{
	time.January: "Jan", time.February: "Feb", time.March: "Maret",
	time.April: "April", time.May: "Mei", time.June: "Juni",
	time.July: "Juli", time.August: "Agu", time.September: "Sept",
	time.October: "Okt", time.November: "Nov", time.December: "Des",
}

// Column layout mirrors the original spreadsheet (A-J) so the numbers
// land where the business already looks for them. K onwards are the
// additions the bot can provide but the manual sheet never had.
var exportHeaders = []string{
	"Tgl", "Pemesan", "Pemakai", "Nmr HP", "Mobil",
	"Driver", "Plat Nmr", "Rekanan", "Jptan", "Keterangan",
	"Jemput", "Kembali", "Status", "Order",
}

var exportWidths = []float64{
	9.1, 14.3, 28.0, 15.0, 13.0,
	11.0, 13.4, 11.1, 9.1, 45.3,
	16.0, 16.0, 11.0, 7.0,
}

// GenerateOrdersExcel builds the workbook: one sheet per month in the
// original layout, plus a summary sheet the manual version never had.
func (s *ExportService) GenerateOrdersExcel() ([]byte, error) {
	var orders []models.Order
	if err := s.DB.Preload("Car").Preload("Driver").
		Order("pickup_datetime asc").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("gagal mengambil data order: %w", err)
	}

	f := excelize.NewFile()

	styles, err := buildStyles(f)
	if err != nil {
		return nil, err
	}

	// Group by calendar month of pickup, in WIB.
	type monthKey struct {
		year  int
		month time.Month
	}
	grouped := map[monthKey][]models.Order{}
	for _, o := range orders {
		t := o.PickupDatetime.In(parser.JakartaLocation)
		k := monthKey{t.Year(), t.Month()}
		grouped[k] = append(grouped[k], o)
	}

	keys := make([]monthKey, 0, len(grouped))
	for k := range grouped {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if keys[i].year != keys[j].year {
			return keys[i].year < keys[j].year
		}
		return keys[i].month < keys[j].month
	})

	if err := s.writeSummarySheet(f, styles, orders); err != nil {
		return nil, err
	}

	for _, k := range keys {
		name := fmt.Sprintf("%s'%02d", monthShort[k.month], k.year%100)
		if _, err := f.NewSheet(name); err != nil {
			return nil, err
		}
		if err := writeMonthSheet(f, styles, name, grouped[k]); err != nil {
			return nil, err
		}
	}

	// Sheet1 is excelize's default and is unused once real sheets exist.
	if len(keys) > 0 || len(orders) >= 0 {
		if idx, err := f.GetSheetIndex("Sheet1"); err == nil && idx != -1 {
			f.DeleteSheet("Sheet1")
		}
	}
	if idx, err := f.GetSheetIndex("Ringkasan"); err == nil && idx != -1 {
		f.SetActiveSheet(idx)
	}

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("gagal membuat file excel: %w", err)
	}
	return buf.Bytes(), nil
}

type exportStyles struct {
	header    int
	title     int
	cell      int
	cellWrap  int
	dateCell  int
	statusOK  int
	statusBad int
	sumLabel  int
	sumValue  int
}

func buildStyles(f *excelize.File) (*exportStyles, error) {
	thin := []excelize.Border{
		{Type: "left", Color: "D0D0D0", Style: 1},
		{Type: "right", Color: "D0D0D0", Style: 1},
		{Type: "top", Color: "D0D0D0", Style: 1},
		{Type: "bottom", Color: "D0D0D0", Style: 1},
	}

	header, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Bold: true, Color: "FFFFFF", Size: 11},
		Fill:      excelize.Fill{Type: "pattern", Color: []string{"1F4E79"}, Pattern: 1},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center", WrapText: true},
		Border:    thin,
	})
	if err != nil {
		return nil, err
	}

	title, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true, Size: 14, Color: "1F4E79"},
	})
	if err != nil {
		return nil, err
	}

	cell, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center"},
		Border:    thin,
	})
	if err != nil {
		return nil, err
	}

	cellWrap, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Vertical: "center", WrapText: true},
		Border:    thin,
	})
	if err != nil {
		return nil, err
	}

	dateCell, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    thin,
	})
	if err != nil {
		return nil, err
	}

	statusOK, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "1E7B34"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    thin,
	})
	if err != nil {
		return nil, err
	}

	statusBad, err := f.NewStyle(&excelize.Style{
		Font:      &excelize.Font{Color: "B00020"},
		Alignment: &excelize.Alignment{Horizontal: "center", Vertical: "center"},
		Border:    thin,
	})
	if err != nil {
		return nil, err
	}

	sumLabel, err := f.NewStyle(&excelize.Style{
		Font: &excelize.Font{Bold: true},
	})
	if err != nil {
		return nil, err
	}

	sumValue, err := f.NewStyle(&excelize.Style{
		Alignment: &excelize.Alignment{Horizontal: "right"},
	})
	if err != nil {
		return nil, err
	}

	return &exportStyles{
		header: header, title: title, cell: cell, cellWrap: cellWrap,
		dateCell: dateCell, statusOK: statusOK, statusBad: statusBad,
		sumLabel: sumLabel, sumValue: sumValue,
	}, nil
}

func writeMonthSheet(f *excelize.File, st *exportStyles, sheet string, orders []models.Order) error {
	for i, h := range exportHeaders {
		cell, _ := excelize.CoordinatesToCellName(i+1, 1)
		f.SetCellValue(sheet, cell, h)
	}
	f.SetCellStyle(sheet, "A1", mustCell(len(exportHeaders), 1), st.header)
	f.SetRowHeight(sheet, 1, 22)

	for i, w := range exportWidths {
		col, _ := excelize.ColumnNumberToName(i + 1)
		f.SetColWidth(sheet, col, col, w)
	}

	for i, o := range orders {
		row := i + 2
		pickup := o.PickupDatetime.In(parser.JakartaLocation)
		ret := o.ReturnDatetime.In(parser.JakartaLocation)

		values := []any{
			fmt.Sprintf("%02d - %02d", pickup.Day(), ret.Day()),
			o.Pemesan,
			o.CustomerName,
			o.CustomerPhone,
			o.Car.Model,
			o.Driver.Name,
			o.Car.PlateNumber,
			o.Partner,
			o.PickupPoint,
			notesWithDestination(o),
			pickup.Format("02 Jan 15:04"),
			ret.Format("02 Jan 15:04"),
			statusLabel(o.Status),
			o.ID,
		}

		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			f.SetCellValue(sheet, cell, v)
		}

		f.SetCellStyle(sheet, mustCell(1, row), mustCell(len(values), row), st.cell)
		f.SetCellStyle(sheet, mustCell(1, row), mustCell(1, row), st.dateCell)
		f.SetCellStyle(sheet, mustCell(10, row), mustCell(10, row), st.cellWrap)
		f.SetCellStyle(sheet, mustCell(11, row), mustCell(12, row), st.dateCell)

		statusStyle := st.statusOK
		if o.Status == models.OrderCancelled {
			statusStyle = st.statusBad
		}
		f.SetCellStyle(sheet, mustCell(13, row), mustCell(14, row), statusStyle)
	}

	// Freeze the header and enable filtering — the manual sheet had
	// neither, which makes 180-row months hard to scan.
	if err := f.SetPanes(sheet, &excelize.Panes{
		Freeze:      true,
		Split:       false,
		XSplit:      0,
		YSplit:      1,
		TopLeftCell: "A2",
		ActivePane:  "bottomLeft",
	}); err != nil {
		return err
	}

	if len(orders) > 0 {
		last := mustCell(len(exportHeaders), len(orders)+1)
		if err := f.AutoFilter(sheet, "A1:"+last, []excelize.AutoFilterOptions{}); err != nil {
			return err
		}
	}

	return nil
}

func (s *ExportService) writeSummarySheet(f *excelize.File, st *exportStyles, orders []models.Order) error {
	const sheet = "Ringkasan"
	if _, err := f.NewSheet(sheet); err != nil {
		return err
	}

	f.SetColWidth(sheet, "A", "A", 28)
	f.SetColWidth(sheet, "B", "B", 14)
	f.SetColWidth(sheet, "C", "C", 4)
	f.SetColWidth(sheet, "D", "D", 28)
	f.SetColWidth(sheet, "E", "E", 14)

	f.SetCellValue(sheet, "A1", "Ringkasan Data Pemakaian Mobil")
	f.SetCellStyle(sheet, "A1", "A1", st.title)
	f.SetCellValue(sheet, "A2", "Dibuat: "+time.Now().In(parser.JakartaLocation).Format("2 Jan 2006 15:04")+" WIB")

	var active, completed, cancelled int
	perCar := map[string]int{}
	perDriver := map[string]int{}
	perAgent := map[string]int{}
	perMonth := map[string]int{}

	for _, o := range orders {
		switch o.Status {
		case models.OrderActive:
			active++
		case models.OrderCompleted:
			completed++
		case models.OrderCancelled:
			cancelled++
		}
		if o.Car.PlateNumber != "" {
			perCar[fmt.Sprintf("%s (%s)", o.Car.PlateNumber, o.Car.Model)]++
		}
		if o.Driver.Name != "" {
			perDriver[o.Driver.Name]++
		}
		if o.Pemesan != "" {
			perAgent[o.Pemesan]++
		}
		t := o.PickupDatetime.In(parser.JakartaLocation)
		perMonth[fmt.Sprintf("%s'%02d", monthShort[t.Month()], t.Year()%100)]++
	}

	row := 4
	writeKV := func(label string, value any) {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), label)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.sumLabel)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), value)
		f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), st.sumValue)
		row++
	}

	writeKV("Total order", len(orders))
	writeKV("Aktif", active)
	writeKV("Selesai", completed)
	writeKV("Dibatalkan", cancelled)

	row++
	writeSection := func(title string, data map[string]int, limit int) {
		f.SetCellValue(sheet, fmt.Sprintf("A%d", row), title)
		f.SetCellStyle(sheet, fmt.Sprintf("A%d", row), fmt.Sprintf("A%d", row), st.header)
		f.SetCellValue(sheet, fmt.Sprintf("B%d", row), "Jumlah")
		f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), st.header)
		row++

		type kv struct {
			k string
			v int
		}
		list := make([]kv, 0, len(data))
		for k, v := range data {
			list = append(list, kv{k, v})
		}
		sort.Slice(list, func(i, j int) bool {
			if list[i].v != list[j].v {
				return list[i].v > list[j].v
			}
			return list[i].k < list[j].k
		})
		if len(list) == 0 {
			f.SetCellValue(sheet, fmt.Sprintf("A%d", row), "(belum ada data)")
			row++
		}
		for i, e := range list {
			if limit > 0 && i >= limit {
				break
			}
			f.SetCellValue(sheet, fmt.Sprintf("A%d", row), e.k)
			f.SetCellValue(sheet, fmt.Sprintf("B%d", row), e.v)
			f.SetCellStyle(sheet, fmt.Sprintf("B%d", row), fmt.Sprintf("B%d", row), st.sumValue)
			row++
		}
		row++
	}

	writeSection("Order per bulan", perMonth, 0)
	writeSection("Pemakaian per mobil", perCar, 0)
	writeSection("Trip per driver", perDriver, 0)
	writeSection("Order per pemesan", perAgent, 15)

	return nil
}

// notesWithDestination keeps the Keterangan column useful whether the
// booking recorded a destination, free-text notes, or both.
func notesWithDestination(o models.Order) string {
	switch {
	case o.Notes != "" && o.DestinationCity != "":
		return o.DestinationCity + " — " + o.Notes
	case o.Notes != "":
		return o.Notes
	default:
		return o.DestinationCity
	}
}

func statusLabel(s models.OrderStatus) string {
	switch s {
	case models.OrderActive:
		return "Aktif"
	case models.OrderCompleted:
		return "Selesai"
	case models.OrderCancelled:
		return "Batal"
	default:
		return string(s)
	}
}

func mustCell(col, row int) string {
	c, _ := excelize.CoordinatesToCellName(col, row)
	return c
}
