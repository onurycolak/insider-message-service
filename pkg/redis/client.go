package redis

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/onurcolak/insider-message-service/environments"
	"github.com/onurcolak/insider-message-service/internal/domain"
	"github.com/onurcolak/insider-message-service/pkg/logger"
	"github.com/valkey-io/valkey-go"
)

type Client struct {
	client valkey.Client
}

const (
	sentMessageKeyPrefix = "sent_message:"
	sentMessageTTL       = 24 * time.Hour
)

func NewRedisClient(cfg environments.RedisConfig) (*Client, error) {
	client, err := valkey.NewClient(valkey.ClientOption{
		InitAddress: []string{fmt.Sprintf("%s:%s", cfg.Host, cfg.Port)},
		Password:    cfg.Password,
		SelectDB:    cfg.DB,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Valkey client: %w", err)
	}

	// Test connection
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := client.Do(ctx, client.B().Ping().Build()).Error(); err != nil {
		client.Close()

		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Infof("Connected to Redis (via Valkey client)")

	return &Client{client: client}, nil
}

func (c *Client) CacheSentMessage(ctx context.Context, dbID int64, messageID string, sentAt time.Time) error {
	cache := domain.SentMessageCache{
		MessageID: messageID,
		SentAt:    sentAt,
	}

	data, err := json.Marshal(cache)
	if err != nil {
		return fmt.Errorf("failed to marshal cache data: %w", err)
	}

	key := fmt.Sprintf("%s%d", sentMessageKeyPrefix, dbID)
	ttlSeconds := int64(sentMessageTTL.Seconds())

	err = c.client.Do(ctx, c.client.B().Set().Key(key).Value(string(data)).Ex(time.Duration(ttlSeconds)*time.Second).Build()).Error()
	if err != nil {
		return fmt.Errorf("failed to cache sent message: %w", err)
	}

	logger.Debugf("Cached message ID %d -> %s in Redis", dbID, messageID)

	return nil
}

func (c *Client) GetCachedMessage(ctx context.Context, dbID int64) (*domain.SentMessageCache, error) {
	key := fmt.Sprintf("%s%d", sentMessageKeyPrefix, dbID)

	result := c.client.Do(ctx, c.client.B().Get().Key(key).Build())
	if result.Error() != nil {
		if valkey.IsValkeyNil(result.Error()) {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to get cached message: %w", result.Error())
	}

	data, err := result.ToString()
	if err != nil {
		return nil, fmt.Errorf("failed to read cached message: %w", err)
	}

	var cache domain.SentMessageCache
	if err := json.Unmarshal([]byte(data), &cache); err != nil {
		return nil, fmt.Errorf("failed to unmarshal cache data: %w", err)
	}

	return &cache, nil
}

func (c *Client) GetAllCachedMessages(ctx context.Context) (map[int64]*domain.SentMessageCache, error) {
	pattern := fmt.Sprintf("%s*", sentMessageKeyPrefix)

	var keys []string
	var cursor uint64
	for {
		result := c.client.Do(ctx, c.client.B().Scan().Cursor(cursor).Match(pattern).Count(100).Build())
		if result.Error() != nil {
			return nil, fmt.Errorf("failed to scan cache keys: %w", result.Error())
		}

		scanResult, err := result.AsScanEntry()
		if err != nil {
			return nil, fmt.Errorf("failed to parse scan result: %w", err)
		}

		keys = append(keys, scanResult.Elements...)
		cursor = scanResult.Cursor

		if cursor == 0 {
			break
		}
	}

	result := make(map[int64]*domain.SentMessageCache)

	for _, key := range keys {
		getResult := c.client.Do(ctx, c.client.B().Get().Key(key).Build())
		if getResult.Error() != nil {
			continue
		}

		data, err := getResult.ToString()
		if err != nil {
			continue
		}

		var cache domain.SentMessageCache
		if err := json.Unmarshal([]byte(data), &cache); err != nil {
			continue
		}

		var dbID int64

		if _, err := fmt.Sscanf(key, sentMessageKeyPrefix+"%d", &dbID); err != nil {
			logger.Warnf("failed to parse dbID from redis key %q: %v", key, err)
			continue
		}

		result[dbID] = &cache
	}

	return result, nil
}

func (c *Client) Close() error {
	c.client.Close()
	return nil
}

func (c *Client) Ping(ctx context.Context) error {
	return c.client.Do(ctx, c.client.B().Ping().Build()).Error()
}
