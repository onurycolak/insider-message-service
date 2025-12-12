package service

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/internal/domain"
)

//
// Test fakes – only for this file.
//

type fakeRepo struct {
	unsent          []domain.Message
	markSentCalls   []markSentCall
	markFailedCalls []int64
	replayByIDCalls []int64
	replayAllCalls  int
	replayAllResult int64
}

type markSentCall struct {
	id        int64
	messageID string
	sentAt    time.Time
}

func (r *fakeRepo) GetUnsent(ctx context.Context, limit int) ([]domain.Message, error) {
	if len(r.unsent) <= limit {
		return r.unsent, nil
	}
	return r.unsent[:limit], nil
}

func (r *fakeRepo) MarkAsSent(ctx context.Context, id int64, messageID string, sentAt time.Time) error {
	r.markSentCalls = append(r.markSentCalls, markSentCall{
		id:        id,
		messageID: messageID,
		sentAt:    sentAt,
	})
	return nil
}

func (r *fakeRepo) MarkAsFailed(ctx context.Context, id int64) error {
	r.markFailedCalls = append(r.markFailedCalls, id)
	return nil
}

// The remaining methods are not used in these tests; we return neutral values.

func (r *fakeRepo) GetSent(ctx context.Context, page, pageSize int) ([]domain.Message, int64, error) {
	return nil, 0, nil
}

func (r *fakeRepo) Create(ctx context.Context, content, phoneNumber string) (*domain.Message, error) {
	return nil, nil
}

func (r *fakeRepo) GetAll(
	ctx context.Context,
	status *domain.MessageStatus,
	page,
	pageSize int,
) ([]domain.Message, int64, error) {
	return nil, 0, nil
}

func (r *fakeRepo) GetStats(ctx context.Context) (pending, sent, failed int64, err error) {
	return 0, 0, 0, nil
}

type fakeWebhookClient struct {
	shouldFail        bool
	responseMessageID string

	lastPhone   string
	lastContent string
}

func (c *fakeWebhookClient) SendMessage(
	ctx context.Context,
	phoneNumber,
	content string,
) (*domain.WebhookResponse, error) {
	c.lastPhone = phoneNumber
	c.lastContent = content

	if c.shouldFail {
		return nil, fmt.Errorf("simulated webhook error")
	}

	messageID := c.responseMessageID
	if messageID == "" {
		messageID = "test-message-id"
	}

	return &domain.WebhookResponse{
		Message:   "Accepted",
		MessageID: messageID,
	}, nil
}

type fakeRedisClient struct {
	cache map[int64]*domain.SentMessageCache
}

func (c *fakeRedisClient) CacheSentMessage(ctx context.Context, dbID int64, messageID string, sentAt time.Time) error {
	if c.cache == nil {
		c.cache = make(map[int64]*domain.SentMessageCache)
	}
	c.cache[dbID] = &domain.SentMessageCache{
		MessageID: messageID,
		SentAt:    sentAt,
	}
	return nil
}

func (c *fakeRedisClient) GetAllCachedMessages(ctx context.Context) (map[int64]*domain.SentMessageCache, error) {
	return c.cache, nil
}

//
// Tests
//

func TestProcessUnsentMessages_SuccessFlow(t *testing.T) {
	ctx := context.Background()

	repo := &fakeRepo{
		unsent: []domain.Message{
			{
				ID:          1,
				Content:     "Hello from Insider",
				PhoneNumber: "+905551234567",
				Status:      domain.StatusPending,
			},
		},
	}

	webhook := &fakeWebhookClient{
		shouldFail:        false,
		responseMessageID: "msg-123",
	}

	redisClient := &fakeRedisClient{}

	cfg := environments.MessageConfig{
		BatchSize:        2,
		SendInterval:     2 * time.Minute,
		MaxContentLength: 1000,
	}

	svc := NewMessageService(repo, webhook, redisClient, cfg)

	results, err := svc.ProcessUnsentMessages(ctx, 0.0)
	if err != nil {
		t.Fatalf("ProcessUnsentMessages returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	res := results[0]
	if !res.Success {
		t.Fatalf("expected Success=true, got false (error: %v)", res.Error)
	}

	if res.MessageID != "msg-123" {
		t.Fatalf("expected MessageID %q, got %q", "msg-123", res.MessageID)
	}

	if len(repo.markSentCalls) != 1 {
		t.Fatalf("expected MarkAsSent to be called once, got %d calls", len(repo.markSentCalls))
	}

	call := repo.markSentCalls[0]
	if call.id != 1 {
		t.Errorf("expected MarkAsSent to be called with id=1, got %d", call.id)
	}
	if call.messageID != "msg-123" {
		t.Errorf("expected MarkAsSent messageID=%q, got %q", "msg-123", call.messageID)
	}

	if redisClient.cache == nil {
		t.Fatalf("expected Redis cache to be initialized")
	}
	if _, ok := redisClient.cache[1]; !ok {
		t.Fatalf("expected message id 1 to be cached in Redis")
	}
}

func TestProcessUnsentMessages_WebhookFailureMarksFailed(t *testing.T) {
	ctx := context.Background()

	repo := &fakeRepo{
		unsent: []domain.Message{
			{
				ID:          42,
				Content:     "This will fail",
				PhoneNumber: "+905551234567",
				Status:      domain.StatusPending,
			},
		},
	}

	webhook := &fakeWebhookClient{
		shouldFail: true,
	}

	redisClient := &fakeRedisClient{}
	cfg := environments.MessageConfig{
		BatchSize:        2,
		SendInterval:     2 * time.Minute,
		MaxContentLength: 1000,
	}

	svc := NewMessageService(repo, webhook, redisClient, cfg)

	results, err := svc.ProcessUnsentMessages(ctx, 0.0)
	if err != nil {
		t.Fatalf("ProcessUnsentMessages returned error: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("expected 1 result, got %d", len(results))
	}

	res := results[0]
	if res.Success {
		t.Fatalf("expected Success=false, got true")
	}

	if len(repo.markFailedCalls) != 1 {
		t.Fatalf("expected MarkAsFailed to be called once, got %d calls", len(repo.markFailedCalls))
	}

	if repo.markFailedCalls[0] != 42 {
		t.Errorf("expected MarkAsFailed to be called with id=42, got %d", repo.markFailedCalls[0])
	}

	// On failure, Redis should not be updated
	if len(redisClient.cache) > 0 {
		t.Fatalf("expected no Redis cache entries on failure, got %d", len(redisClient.cache))
	}
}

func TestProcessUnsentMessages_ContentTruncation(t *testing.T) {
	ctx := context.Background()

	// Content longer than MaxContentLength
	longContent := "0123456789ABCDEFGHIJ" // 20 chars

	repo := &fakeRepo{
		unsent: []domain.Message{
			{
				ID:          7,
				Content:     longContent,
				PhoneNumber: "+905551234567",
				Status:      domain.StatusPending,
			},
		},
	}

	webhook := &fakeWebhookClient{
		shouldFail:        false,
		responseMessageID: "msg-long",
	}

	redisClient := &fakeRedisClient{}

	// Force truncation to a small length to make the behaviour obvious
	cfg := environments.MessageConfig{
		BatchSize:        2,
		SendInterval:     2 * time.Minute,
		MaxContentLength: 10,
	}

	svc := NewMessageService(repo, webhook, redisClient, cfg)

	_, err := svc.ProcessUnsentMessages(ctx, 0.0)
	if err != nil {
		t.Fatalf("ProcessUnsentMessages returned error: %v", err)
	}

	// With MaxContentLength=10 and "..." suffix,
	// expected content is first 7 chars + "..."
	expected := "0123456..."

	if webhook.lastContent != expected {
		t.Fatalf("expected truncated content %q, got %q", expected, webhook.lastContent)
	}
}

func TestCreateMessage_ContentTooLong(t *testing.T) {
	ctx := context.Background()

	repo := &fakeRepo{}
	webhook := &fakeWebhookClient{}
	redisClient := &fakeRedisClient{}

	cfg := environments.MessageConfig{
		BatchSize:        2,
		SendInterval:     2 * time.Minute,
		MaxContentLength: 10,
	}

	svc := NewMessageService(repo, webhook, redisClient, cfg)

	longContent := "0123456789ABC" // 13 > 10
	_, err := svc.CreateMessage(ctx, longContent, "+905551234567")
	if err == nil {
		t.Fatalf("expected error for too-long content, got nil")
	}

	expectedErr := "content exceeds maximum length of 10 characters"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestGetCachedMessages_NoRedisConfigured(t *testing.T) {
	ctx := context.Background()

	repo := &fakeRepo{}
	webhook := &fakeWebhookClient{}

	cfg := environments.MessageConfig{
		BatchSize:        2,
		SendInterval:     2 * time.Minute,
		MaxContentLength: 1000,
	}

	// Construct service with nil redis client
	svc := NewMessageService(repo, webhook, nil, cfg)

	cached, err := svc.GetCachedMessages(ctx)
	if err == nil {
		t.Fatalf("expected error when redis client is nil, got nil")
	}

	expectedErr := "redis client not configured"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}

	if cached != nil {
		t.Fatalf("expected cached result to be nil on error, got %#v", cached)
	}
}

func (r *fakeRepo) ReplayFailedByID(ctx context.Context, id int64) error {
	r.replayByIDCalls = append(r.replayByIDCalls, id)

	return nil
}

func (r *fakeRepo) ReplayAllFailed(ctx context.Context) (int64, error) {
	r.replayAllCalls++

	return r.replayAllResult, nil
}

func TestReplayAllFailedMessages_DelegatesToRepo(t *testing.T) {
	ctx := context.Background()

	repo := &fakeRepo{
		replayAllResult: 5, // pretend repo will replay 5 rows
	}
	webhook := &fakeWebhookClient{}
	redisClient := &fakeRedisClient{}

	cfg := environments.MessageConfig{
		BatchSize:        2,
		SendInterval:     2 * time.Minute,
		MaxContentLength: 1000,
	}

	svc := NewMessageService(repo, webhook, redisClient, cfg)

	count, err := svc.ReplayAllFailedMessages(ctx)
	if err != nil {
		t.Fatalf("ReplayAllFailedMessages returned error: %v", err)
	}

	if repo.replayAllCalls != 1 {
		t.Fatalf("expected ReplayAllFailed to be called once, got %d", repo.replayAllCalls)
	}

	if count != 5 {
		t.Fatalf("expected replay count 5, got %d", count)
	}
}

func TestReplayFailedMessage_DelegatesToRepo(t *testing.T) {
	ctx := context.Background()

	repo := &fakeRepo{}
	webhook := &fakeWebhookClient{}
	redisClient := &fakeRedisClient{}

	cfg := environments.MessageConfig{
		BatchSize:        2,
		SendInterval:     2 * time.Minute,
		MaxContentLength: 1000,
	}

	svc := NewMessageService(repo, webhook, redisClient, cfg)

	const id int64 = 42

	if err := svc.ReplayFailedMessage(ctx, id); err != nil {
		t.Fatalf("ReplayFailedMessage returned error: %v", err)
	}

	if len(repo.replayByIDCalls) != 1 {
		t.Fatalf("expected ReplayFailedByID to be called once, got %d", len(repo.replayByIDCalls))
	}

	if repo.replayByIDCalls[0] != id {
		t.Fatalf("expected ReplayFailedByID to be called with id=%d, got %d", id, repo.replayByIDCalls[0])
	}
}
