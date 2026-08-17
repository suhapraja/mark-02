# mark-02 — Car Rental WhatsApp Assistant

A private WhatsApp bot for a car rental business. Dad checks car/driver
availability and logs orders by chatting with the bot; drivers report trip
completion and location the same way. Built for internal use only — not
customer-facing.

See `rental-bot-plan.md` (in project docs) for the full flow and design
decisions.

## Stack

- Go (`chi` router, `gorm`)
- PostgreSQL
- WhatsApp Cloud API (official, Meta)
- `excelize` for Excel export

## Project Layout

```
cmd/
  server/   — main application entrypoint (webhook server)
  seed/     — one-off script to load the fixed car/driver list
internal/
  config/   — environment variable loading
  db/       — database connection + auto-migration
  models/   — Car, Driver, Staff, Order, LocationLog
  parser/   — parses Indonesian WhatsApp commands into structured data
  service/  — business logic (availability, booking, export)
  whatsapp/ — WhatsApp Cloud API client + webhook payload types
  handlers/ — webhook HTTP handlers, ties parser + service + whatsapp together
seed/       — cars.json / drivers.json / staff.json — fill in with real data
```

## Setup

1. **Copy `.env.example` to `.env`** and fill in the values (see comments
   in that file for where each one comes from).

2. **Set up PostgreSQL** — locally via Docker, or use whatever your
   hosting platform provisions.

3. **Fill in `seed/cars.json`, `seed/drivers.json` and `seed/staff.json`**
   with the real cars, drivers and staff, then run:

   ```
   go run ./cmd/seed
   ```

4. **Run the server locally:**

   ```
   go run ./cmd/server
   ```

   It listens on `PORT` (default 8080) and exposes:
   - `GET /health` — basic health check
   - `GET /webhook` — Meta's verification challenge endpoint
   - `POST /webhook` — receives incoming WhatsApp messages

## Roles

The bot decides how to read a message purely from the sender's number.

| Role | Who | Can do |
|---|---|---|
| **Superadmin** | Business owner, developer | Everything admins can, plus manage staff/drivers and car maintenance |
| **Admin** | Trusted staff | Availability, bookings, cancel, edit, export |
| **Driver** | Drivers | Report trip completion and location |

A number can be staff *or* a driver, never both — registering the same
number twice is rejected.

Superadmins listed in `SUPERADMIN_PHONES` are re-applied on every startup,
so there is always a way back in. Everyone else is managed from WhatsApp,
and the last remaining superadmin cannot be removed.

## Commands (WhatsApp)

**Admin and superadmin:**

```
cek mobil 20 Agustus 08:00 - 22 Agustus 17:00
booking Avanza B1234, Budi, 20 Agustus 08:00 - 22 Agustus 17:00, customer Yusuf, tujuan Bandung
batal 12
ubah 12, driver Andi
ubah 12, mobil Innova B5678
ubah 12, waktu 21 Agustus 09:00 - 23 Agustus 17:00
export
help
```

**Superadmin only:**

```
tambah admin Budi, 08123456789
tambah driver Andi, 08123456790
hapus admin 08123456789
hapus driver 08123456790
daftar staff
maintenance Avanza B1234
siap Avanza B1234
```

Phone numbers are accepted in any common format — `08123456789`,
`+62 812-3456-789` and `628123456789` all resolve to the same number.

**Driver:**

```
selesai, sekarang di Bandung
posisi Jakarta
```

## Deployment (Railway/Render)

Not deployed yet — next step once the code is reviewed. Both platforms
support deploying a Go app directly from this GitHub repo with a
PostgreSQL add-on, and let you set the environment variables from
`.env.example` in their dashboard.

You'll also need, outside of this repo:
- A Meta Business Manager account
- WhatsApp Business Cloud API access with a dedicated phone number
  (separate from dad's personal WhatsApp)
- The webhook URL (once deployed) registered in the Meta App dashboard,
  using `WHATSAPP_VERIFY_TOKEN` from your `.env` during verification
