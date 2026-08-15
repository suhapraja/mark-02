package service

import (
	"fmt"

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

// GenerateOrdersExcel builds an Excel workbook of all orders (all statuses)
// and returns the raw file bytes, ready to save or send.
func (s *ExportService) GenerateOrdersExcel() ([]byte, error) {
	var orders []models.Order
	if err := s.DB.Preload("Car").Preload("Driver").
		Order("pickup_datetime desc").Find(&orders).Error; err != nil {
		return nil, fmt.Errorf("gagal mengambil data order: %w", err)
	}

	f := excelize.NewFile()
	sheet := "Orders"
	f.SetSheetName("Sheet1", sheet)

	headers := []string{
		"Order ID", "Status", "Customer", "No. HP Customer",
		"Mobil", "Driver", "Jemput", "Kembali", "Tujuan",
		"Dibuat", "Terakhir Diubah",
	}
	for col, h := range headers {
		cell, _ := excelize.CoordinatesToCellName(col+1, 1)
		f.SetCellValue(sheet, cell, h)
	}

	const layout = "2006-01-02 15:04"
	for i, o := range orders {
		row := i + 2
		values := []any{
			o.ID,
			string(o.Status),
			o.CustomerName,
			o.CustomerPhone,
			fmt.Sprintf("%s (%s)", o.Car.PlateNumber, o.Car.Model),
			o.Driver.Name,
			o.PickupDatetime.In(parser.JakartaLocation).Format(layout),
			o.ReturnDatetime.In(parser.JakartaLocation).Format(layout),
			o.DestinationCity,
			o.CreatedAt.In(parser.JakartaLocation).Format(layout),
			o.LastEditedAt.In(parser.JakartaLocation).Format(layout),
		}
		for col, v := range values {
			cell, _ := excelize.CoordinatesToCellName(col+1, row)
			f.SetCellValue(sheet, cell, v)
		}
	}

	f.SetColWidth(sheet, "A", "K", 18)

	buf, err := f.WriteToBuffer()
	if err != nil {
		return nil, fmt.Errorf("gagal membuat file excel: %w", err)
	}
	return buf.Bytes(), nil
}
