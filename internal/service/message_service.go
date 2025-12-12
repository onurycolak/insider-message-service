package service

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/internal/domain"
	"github.com/onurcolak/insider-message-service/pkg/logger"
)

// Small internal interfaces so we can test without touching real DB/Redis/webhook.
type messageRepository interface {
	GetUnsent(ctx context.Context, limit int) ([]domain.Message, error)
	MarkAsSent(ctx context.Context, id int64, messageID string, sentAt time.Time) error
	MarkAsFailed(ctx context.Context, id int64) error

	GetSent(ctx context.Context, page, pageSize int) ([]domain.Message, int64, error)
	Create(ctx context.Context, content, phoneNumber string) (*domain.Message, error)
	GetAll(ctx context.Context, status *domain.MessageStatus, page, pageSize int) ([]domain.Message, int64, error)
	GetStats(ctx context.Context) (pending, sent, failed int64, err error)

	// new
	ReplayFailedByID(ctx context.Context, id int64) error
	ReplayAllFailed(ctx context.Context) (int64, error)
}

type webhookClient interface {
	SendMessage(ctx context.Context, phoneNumber, content string) (*domain.WebhookResponse, error)
}

type redisClient interface {
	CacheSentMessage(ctx context.Context, dbID int64, messageID string, sentAt time.Time) error
	GetAllCachedMessages(ctx context.Context) (map[int64]*domain.SentMessageCache, error)
}

type MessageService struct {
	repo          messageRepository
	webhookClient webhookClient
	redisClient   redisClient
	config        environments.MessageConfig
}

func NewMessageService(
	repo messageRepository,
	webhookClient webhookClient,
	redisClient redisClient,
	config environments.MessageConfig,
) *MessageService {
	return &MessageService{
		repo:          repo,
		webhookClient: webhookClient,
		redisClient:   redisClient,
		config:        config,
	}
}

func (s *MessageService) ProcessUnsentMessages(ctx context.Context, failureRate float64) ([]domain.SendResult, error) {
	messages, err := s.repo.GetUnsent(ctx, s.config.BatchSize)
	if err != nil {
		return nil, fmt.Errorf("failed to get unsent messages: %w", err)
	}

	if len(messages) == 0 {
		logger.Debugf("No unsent messages to process")
		return nil, nil
	}

	logger.Infof("Processing %d unsent messages", len(messages))

	results := make([]domain.SendResult, 0, len(messages))

	for _, msg := range messages {
		shouldFail := rand.Float64() < failureRate

		result := s.deliverMessage(ctx, &msg, shouldFail)
		results = append(results, result)
	}

	return results, nil
}

func (s *MessageService) deliverMessage(
	ctx context.Context,
	msg *domain.Message,
	shouldFailAll bool,
) domain.SendResult {
	result := domain.SendResult{
		MessageDBID: msg.ID,
		SentAt:      time.Now(),
	}

	// Simulated failure for testing.
	if shouldFailAll {
		logger.Warnf("Simulated failure for message %d (failure rate test)", msg.ID)

		result.Success = false
		result.Error = fmt.Errorf("simulated failure for testing")

		if markErr := s.repo.MarkAsFailed(ctx, msg.ID); markErr != nil {
			logger.Errorf("Failed to mark message %d as failed: %v", msg.ID, markErr)
		}

		return result
	}

	// Enforce max content length.
	if len(msg.Content) > s.config.MaxContentLength {
		logger.Warnf("Message %d exceeds max content length (%d > %d)",
			msg.ID, len(msg.Content), s.config.MaxContentLength)

		ellipsis := "..."
		max := s.config.MaxContentLength
		if max > len(ellipsis) {
			msg.Content = msg.Content[:max-len(ellipsis)] + ellipsis
		} else {
			msg.Content = msg.Content[:max]
		}
	}

	resp, err := s.webhookClient.SendMessage(ctx, msg.PhoneNumber, msg.Content)
	if err != nil {
		logger.Errorf("Failed to send message %d: %v", msg.ID, err)
		result.Success = false
		result.Error = err

		if markErr := s.repo.MarkAsFailed(ctx, msg.ID); markErr != nil {
			logger.Errorf("Failed to mark message %d as failed: %v", msg.ID, markErr)
		}

		return result
	}

	if err := s.repo.MarkAsSent(ctx, msg.ID, resp.MessageID, result.SentAt); err != nil {
		logger.Errorf("Failed to mark message %d as sent: %v", msg.ID, err)
		result.Success = false
		result.Error = err
		return result
	}

	if s.redisClient != nil {
		if err := s.redisClient.CacheSentMessage(ctx, msg.ID, resp.MessageID, result.SentAt); err != nil {
			logger.Warnf("Failed to cache message %d to Redis: %v", msg.ID, err)
		}
	}

	logger.Infof("Successfully sent message %d (webhookMessageId: %s)", msg.ID, resp.MessageID)

	result.Success = true
	result.MessageID = resp.MessageID

	return result
}

func (s *MessageService) GetSentMessages(ctx context.Context, page, pageSize int) ([]domain.Message, int64, error) {
	return s.repo.GetSent(ctx, page, pageSize)
}

func (s *MessageService) CreateMessage(ctx context.Context, content, phoneNumber string) (*domain.Message, error) {
	if len(content) > s.config.MaxContentLength {
		return nil, fmt.Errorf("content exceeds maximum length of %d characters", s.config.MaxContentLength)
	}

	return s.repo.Create(ctx, content, phoneNumber)
}

func (s *MessageService) GetAllMessages(
	ctx context.Context,
	status *domain.MessageStatus,
	page,
	pageSize int,
) ([]domain.Message, int64, error) {
	return s.repo.GetAll(ctx, status, page, pageSize)
}

func (s *MessageService) GetStats(ctx context.Context) (pending, sent, failed int64, err error) {
	return s.repo.GetStats(ctx)
}

func (s *MessageService) GetCachedMessages(ctx context.Context) (map[int64]*domain.SentMessageCache, error) {
	if s.redisClient == nil {
		return nil, fmt.Errorf("redis client not configured")
	}
	return s.redisClient.GetAllCachedMessages(ctx)
}

func (s *MessageService) ReplayFailedMessage(ctx context.Context, id int64) error {
	return s.repo.ReplayFailedByID(ctx, id)
}

func (s *MessageService) ReplayAllFailedMessages(ctx context.Context) (int64, error) {
	return s.repo.ReplayAllFailed(ctx)
}
