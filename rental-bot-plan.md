# WhatsApp Car Rental Assistant — Project Plan

## 1. Purpose

A private WhatsApp bot for your dad's car rental business. It replaces mental
tracking of car/driver availability with a system he and his drivers talk to
directly on WhatsApp. Customer orders still come in by phone/WhatsApp as
usual — this bot is the internal tool that sits behind that conversation.

## 2. Roles

| Role | Can do |
|---|---|
| **Admin (Dad)** | Check availability, log new orders, view/export records |
| **Driver** | Report trip completion + current location |

Both talk to the *same* WhatsApp Business number. The bot recognizes who's
messaging by their phone number and responds differently.

## 3. Core Flow

1. Customer orders via phone/WhatsApp → dad (unchanged, human-to-human)
2. Dad → bot: `cek mobil 20-22 Agustus`
   → bot replies with free cars, drivers, and each driver's last known location
3. Dad → bot: `booking Avanza B1234, Budi, 20-22 Agustus, customer Yusuf, tujuan Bandung`
   → bot logs the order, marks car + driver busy for those dates
4. Trip happens
5. Driver → bot: `selesai, sekarang di Bandung`
   → bot marks driver/car available again, updates last location
6. Dad → bot: `export` (any time)
   → bot generates and sends an Excel file of all orders

## 4. Data Model

**Cars**
- car_id, plate number, model, status (available / on_trip / maintenance)

**Drivers**
- driver_id, name, phone number, last_location, status (available / on_trip)

**Orders**
- order_id, car_id, driver_id, customer_name, customer_phone (optional),
  pickup_datetime, return_datetime, destination_city, status (active / completed / cancelled),
  created_at, last_edited_at

**Location log** (optional, for history)
- driver_id, location, timestamp

## 5. Bot Commands (draft — refine wording later)

| Who | Command example | Result |
|---|---|---|
| Dad | `cek mobil [date range]` | Lists free cars + drivers + driver locations |
| Dad | `booking [car], [driver], [dates], [customer], tujuan [city]` | Creates order, marks busy |
| Dad | `batal [order id]` | Cancels an order, frees car/driver |
| Dad | `ubah [order id], [what to change]` | Edits date/time, driver, or car on an existing order |
| Dad | `export` | Sends Excel file of all records |
| Driver | `selesai, sekarang di [city]` | Marks trip complete, updates location |
| Driver | `posisi [city]` | Updates location without ending a trip (idle updates) |

*Bahasa Indonesia phrasing throughout since that's what your dad and drivers
will actually type — command matching should be forgiving (handle typos,
different word order) rather than rigid syntax.*

## 6. Tech Stack

- **WhatsApp Cloud API** (official, Meta) — requires a Meta Business
  account + a dedicated phone number for the bot (not your dad's personal number)
- **Backend**: Go — `net/http` or `chi` router for the webhook, `gorm` (or `sqlc`) for the database layer
- **Database**: PostgreSQL (small, hosted alongside the app)
- **Excel export**: generated on demand with the `excelize` library, sent back as a WhatsApp file attachment
- **Hosting**: Railway or Render — both support one-click deploy from a GitHub repo (Go apps supported natively), free/cheap tier is enough for this volume

## 7. What Needs Setting Up Outside This Chat

These require your accounts/verification — I'll provide exact steps when we get there, but can't do them for you:

1. **Meta Business Manager account** (if you don't already have one)
2. **WhatsApp Business Cloud API access** + a phone number registered to it (separate from your dad's personal number)
3. **Railway/Render account** to host the backend
4. **GitHub account** to hold the code (so the host can deploy from it)

## 8. Confirmed Decisions

- **Conflict checking is time-aware**: pickup/return stored as full date + time, not just date — so same-day back-to-back bookings on one car/driver don't false-positive as conflicts. Dad gets a warning if a booking overlaps an existing one, rather than being silently blocked.
- **Auto-notify driver**: once dad books an order, the bot messages the assigned driver directly with the trip details (customer, pickup time, destination).
- **Edit existing orders**: a command to change the date/time, driver, or car on an existing booking, not just cancel-and-rebook.
- **Maintenance status**: cars can be marked "under maintenance" — a third status alongside available/on_trip — so they're excluded from availability results without needing a fake booking to hide them.

## 9. Next Step

Once the flow and open questions above are confirmed, I'll build:
- The full backend code (ready to deploy)
- Step-by-step deployment guide for Meta + Railway/Render
- A short "cheat sheet" of commands for your dad and drivers
