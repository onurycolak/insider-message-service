package domain

import "time"

type MessageStatus string

const (
	StatusPending MessageStatus = "pending"
	StatusSent    MessageStatus = "sent"
	StatusFailed  MessageStatus = "failed"
)

type Message struct {
	ID          int64         `db:"id" json:"id"`
	Content     string        `db:"content" json:"content"`
	PhoneNumber string        `db:"phone_number" json:"phoneNumber"`
	Status      MessageStatus `db:"status" json:"status"`
	MessageID   *string       `db:"message_id" json:"messageId,omitempty"`
	SentAt      *time.Time    `db:"sent_at" json:"sentAt,omitempty"`
	CreatedAt   time.Time     `db:"created_at" json:"createdAt"`
	UpdatedAt   time.Time     `db:"updated_at" json:"updatedAt"`
}

type SentMessageCache struct {
	MessageID string    `json:"messageId"`
	SentAt    time.Time `json:"sentAt"`
}

type WebhookRequest struct {
	To      string `json:"to"`
	Content string `json:"content"`
}

type WebhookResponse struct {
	Message   string `json:"message"`
	MessageID string `json:"messageId"`
}

type SendResult struct {
	MessageDBID int64
	MessageID   string
	Success     bool
	Error       error
	SentAt      time.Time
}
