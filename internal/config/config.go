package config

import (
	"log"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port                  string
	DatabaseURL           string
	WhatsAppToken         string // Meta Cloud API permanent/temp access token
	WhatsAppPhoneNumberID string // Meta phone number ID for sending messages
	WhatsAppVerifyToken   string // Used to verify the webhook with Meta
	AdminPhone            string // Dad's WhatsApp number, e.g. 62812xxxxxxx
}

func Load() Config {
	// .env is optional (useful for local dev); in production these come
	// from the hosting platform's environment variables.
	if err := godotenv.Load(); err != nil {
		log.Println("no .env file found, relying on environment variables")
	}

	return Config{
		Port:                  getEnv("PORT", "8080"),
		DatabaseURL:           mustGetEnv("DATABASE_URL"),
		WhatsAppToken:         mustGetEnv("WHATSAPP_TOKEN"),
		WhatsAppPhoneNumberID: mustGetEnv("WHATSAPP_PHONE_NUMBER_ID"),
		WhatsAppVerifyToken:   mustGetEnv("WHATSAPP_VERIFY_TOKEN"),
		AdminPhone:            mustGetEnv("ADMIN_PHONE"),
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func mustGetEnv(key string) string {
	v := os.Getenv(key)
	if v == "" {
		log.Fatalf("required environment variable %s is not set", key)
	}
	return v
}
