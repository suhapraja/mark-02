package main

import (
	"log"
	"net/http"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/suhapraja/mark-02/internal/config"
	"github.com/suhapraja/mark-02/internal/db"
	"github.com/suhapraja/mark-02/internal/handlers"
	"github.com/suhapraja/mark-02/internal/service"
	"github.com/suhapraja/mark-02/internal/whatsapp"
)

func main() {
	cfg := config.Load()

	conn := db.Connect(cfg.DatabaseURL)
	if err := db.AutoMigrate(conn); err != nil {
		log.Fatalf("migration failed: %v", err)
	}

	waClient := whatsapp.NewClient(cfg.WhatsAppToken, cfg.WhatsAppPhoneNumberID)

	availabilitySvc := service.NewAvailabilityService(conn)
	bookingSvc := service.NewBookingService(conn, availabilitySvc)
	exportSvc := service.NewExportService(conn)

	webhookHandler := &handlers.WebhookHandler{
		VerifyToken:  cfg.WhatsAppVerifyToken,
		AdminPhone:   cfg.AdminPhone,
		WA:           waClient,
		Availability: availabilitySvc,
		Booking:      bookingSvc,
		Export:       exportSvc,
	}

	r := chi.NewRouter()
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	r.Get("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("ok"))
	})

	r.Get("/webhook", webhookHandler.VerifyWebhook)
	r.Post("/webhook", webhookHandler.ReceiveMessage)

	log.Printf("server listening on port %s", cfg.Port)
	if err := http.ListenAndServe(":"+cfg.Port, r); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
