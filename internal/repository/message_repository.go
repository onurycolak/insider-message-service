package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"

	"github.com/onurcolak/insider-message-service/internal/domain"
)

// MessageRepository handles database operations for messages.
type MessageRepository struct {
	db *sqlx.DB
}

func NewMessageRepository(db *sqlx.DB) *MessageRepository {
	return &MessageRepository{db: db}
}

func (r *MessageRepository) GetUnsent(ctx context.Context, limit int) ([]domain.Message, error) {
	query := `
		SELECT id, content, phone_number, status, message_id, sent_at, created_at, updated_at
		FROM messages
		WHERE status = 'pending'
		ORDER BY created_at ASC
		LIMIT ?
	`

	var messages []domain.Message
	if err := r.db.SelectContext(ctx, &messages, query, limit); err != nil {
		return nil, fmt.Errorf("failed to get unsent messages: %w", err)
	}

	return messages, nil
}

func (r *MessageRepository) MarkAsSent(ctx context.Context, id int64, messageID string, sentAt time.Time) error {
	query := `
		UPDATE messages
		SET status = 'sent', message_id = ?, sent_at = ?, updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	result, err := r.db.ExecContext(ctx, query, messageID, sentAt, id)
	if err != nil {
		return fmt.Errorf("failed to mark message as sent: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no message found with id %d", id)
	}

	return nil
}

func (r *MessageRepository) MarkAsFailed(ctx context.Context, id int64) error {
	query := `
		UPDATE messages
		SET status = 'failed', updated_at = CURRENT_TIMESTAMP
		WHERE id = ?
	`

	_, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to mark message as failed: %w", err)
	}

	return nil
}

func (r *MessageRepository) GetSent(ctx context.Context, page, pageSize int) ([]domain.Message, int64, error) {
	offset := (page - 1) * pageSize

	var totalCount int64
	countQuery := "SELECT COUNT(*) FROM messages WHERE status = 'sent'"
	if err := r.db.GetContext(ctx, &totalCount, countQuery); err != nil {
		return nil, 0, fmt.Errorf("failed to count sent messages: %w", err)
	}

	query := `
		SELECT id, content, phone_number, status, message_id, sent_at, created_at, updated_at
		FROM messages
		WHERE status = 'sent'
		ORDER BY sent_at DESC
		LIMIT ? OFFSET ?
	`

	var messages []domain.Message
	if err := r.db.SelectContext(ctx, &messages, query, pageSize, offset); err != nil {
		return nil, 0, fmt.Errorf("failed to get sent messages: %w", err)
	}

	return messages, totalCount, nil
}

func (r *MessageRepository) GetByID(ctx context.Context, id int64) (*domain.Message, error) {
	query := `
		SELECT id, content, phone_number, status, message_id, sent_at, created_at, updated_at
		FROM messages
		WHERE id = ?
	`

	var message domain.Message
	if err := r.db.GetContext(ctx, &message, query, id); err != nil {
		if err == sql.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get message: %w", err)
	}

	return &message, nil
}

func (r *MessageRepository) Create(ctx context.Context, content, phoneNumber string) (*domain.Message, error) {
	query := `
		INSERT INTO messages (content, phone_number, status, created_at, updated_at)
		VALUES (?, ?, 'pending', CURRENT_TIMESTAMP, CURRENT_TIMESTAMP)
	`

	result, err := r.db.ExecContext(ctx, query, content, phoneNumber)
	if err != nil {
		return nil, fmt.Errorf("failed to create message: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert id: %w", err)
	}

	return r.GetByID(ctx, id)
}

func (r *MessageRepository) GetAll(
	ctx context.Context,
	status *domain.MessageStatus,
	page, pageSize int,
) ([]domain.Message, int64, error) {
	offset := (page - 1) * pageSize
	var totalCount int64
	var messages []domain.Message

	if status != nil {
		countQuery := "SELECT COUNT(*) FROM messages WHERE status = ?"
		if err := r.db.GetContext(ctx, &totalCount, countQuery, *status); err != nil {
			return nil, 0, fmt.Errorf("failed to count messages: %w", err)
		}

		query := `
			SELECT id, content, phone_number, status, message_id, sent_at, created_at, updated_at
			FROM messages
			WHERE status = ?
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		if err := r.db.SelectContext(ctx, &messages, query, *status, pageSize, offset); err != nil {
			return nil, 0, fmt.Errorf("failed to get messages: %w", err)
		}
	} else {
		countQuery := "SELECT COUNT(*) FROM messages"
		if err := r.db.GetContext(ctx, &totalCount, countQuery); err != nil {
			return nil, 0, fmt.Errorf("failed to count messages: %w", err)
		}

		query := `
			SELECT id, content, phone_number, status, message_id, sent_at, created_at, updated_at
			FROM messages
			ORDER BY created_at DESC
			LIMIT ? OFFSET ?
		`
		if err := r.db.SelectContext(ctx, &messages, query, pageSize, offset); err != nil {
			return nil, 0, fmt.Errorf("failed to get messages: %w", err)
		}
	}

	return messages, totalCount, nil
}

// GetStats returns statistics about messages.
func (r *MessageRepository) GetStats(ctx context.Context) (pending, sent, failed int64, err error) {
	query := `
		SELECT 
			COALESCE(SUM(CASE WHEN status = 'pending' THEN 1 ELSE 0 END), 0) AS pending,
			COALESCE(SUM(CASE WHEN status = 'sent' THEN 1 ELSE 0 END), 0)    AS sent,
			COALESCE(SUM(CASE WHEN status = 'failed' THEN 1 ELSE 0 END), 0)  AS failed
		FROM messages
	`

	var stats struct {
		Pending int64 `db:"pending"`
		Sent    int64 `db:"sent"`
		Failed  int64 `db:"failed"`
	}

	if err := r.db.GetContext(ctx, &stats, query); err != nil {
		return 0, 0, 0, fmt.Errorf("failed to get stats: %w", err)
	}

	return stats.Pending, stats.Sent, stats.Failed, nil
}

func (r *MessageRepository) ReplayFailedByID(ctx context.Context, id int64) error {
	query := `
		UPDATE messages
		SET status = 'pending',
		    message_id = NULL,
		    sent_at = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE id = ? AND status = 'failed'
	`

	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to replay failed message: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get affected rows: %w", err)
	}

	if rows == 0 {
		return fmt.Errorf("no failed message found with id %d", id)
	}

	return nil
}

func (r *MessageRepository) ReplayAllFailed(ctx context.Context) (int64, error) {
	query := `
		UPDATE messages
		SET status = 'pending',
		    message_id = NULL,
		    sent_at = NULL,
		    updated_at = CURRENT_TIMESTAMP
		WHERE status = 'failed'
	`

	result, err := r.db.ExecContext(ctx, query)
	if err != nil {
		return 0, fmt.Errorf("failed to replay failed messages: %w", err)
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to get affected rows: %w", err)
	}

	return rows, nil
}
