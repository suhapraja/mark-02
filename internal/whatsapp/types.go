package whatsapp

// Minimal subset of the WhatsApp Cloud API webhook payload — only the
// fields this bot actually needs (incoming text messages, sender number).
// See: https://developers.facebook.com/docs/whatsapp/cloud-api/webhooks

type WebhookPayload struct {
	Entry []Entry `json:"entry"`
}

type Entry struct {
	Changes []Change `json:"changes"`
}

type Change struct {
	Value Value `json:"value"`
}

type Value struct {
	Messages []Message `json:"messages"`
	Contacts []Contact `json:"contacts"`
}

type Contact struct {
	Profile Profile `json:"profile"`
	WaID    string  `json:"wa_id"`
}

type Profile struct {
	Name string `json:"name"`
}

type Message struct {
	From string      `json:"from"` // sender's phone number, e.g. "62812xxxxxxx"
	ID   string      `json:"id"`
	Type string      `json:"type"` // "text", "image", etc — this bot only handles "text"
	Text MessageText `json:"text"`
}

type MessageText struct {
	Body string `json:"body"`
}
