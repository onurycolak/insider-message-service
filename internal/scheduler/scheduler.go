package scheduler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/onurcolak/insider-message-service/internal/domain"
	"github.com/onurcolak/insider-message-service/internal/service"
	"github.com/onurcolak/insider-message-service/pkg/logger"
)

// messageProcessor is a minimal internal interface for the scheduler.
// It matches the ProcessUnsentMessages method of MessageService and
// lets us unit test the scheduler with a small fake implementation.
type messageProcessor interface {
	ProcessUnsentMessages(ctx context.Context, failureRate float64) ([]domain.SendResult, error)
}

type Scheduler struct {
	messageService  messageProcessor
	interval        time.Duration
	failureRate     float64 // Probability of failure (0-1)
	alertWebhook    string
	alertThreshold  int // Number of consecutive all-fail iterations before alert
	lastAlertSentAt time.Time

	// Internal state
	running  bool
	stopChan chan struct{}
	doneChan chan struct{}
	mu       sync.RWMutex

	// Statistics
	lastRunAt    time.Time
	messagesSent int64
	runsCount    int64

	// Alert tracking
	consecutiveAllFailCount int // Count of consecutive iterations where all messages failed
}

func NewScheduler(messageService *service.MessageService, interval time.Duration) *Scheduler {
	return &Scheduler{
		messageService: messageService,
		interval:       interval,
		running:        false,
	}
}

func (s *Scheduler) StartWithParams(
	ctx context.Context,
	intervalMinutes int,
	failureRate float64, alertWebhook string,
	alertThreshold int,
) error {
	if intervalMinutes <= 0 {
		intervalMinutes = 120
	}

	s.mu.Lock()
	s.interval = time.Duration(intervalMinutes) * time.Minute
	s.failureRate = failureRate
	s.alertWebhook = alertWebhook
	s.alertThreshold = alertThreshold
	s.consecutiveAllFailCount = 0
	s.mu.Unlock()

	return s.Start(ctx)
}

func (s *Scheduler) Start(ctx context.Context) error {
	s.mu.Lock()

	if s.running {
		s.mu.Unlock()
		logger.Warnf("Scheduler is already running")
		return nil
	}

	s.running = true
	s.stopChan = make(chan struct{})
	s.doneChan = make(chan struct{})
	s.mu.Unlock()

	logger.Infof("Starting scheduler with interval: %v", s.interval)

	go s.run(ctx)

	return nil
}

func (s *Scheduler) run(ctx context.Context) {
	defer close(s.doneChan)

	s.processMessages(ctx)

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	logger.Infof("Scheduler running. Next execution in %v", s.interval)

	for {
		select {
		case <-ticker.C:
			s.processMessages(ctx)
			logger.Debugf("Next execution in %v", s.interval)

		case <-s.stopChan:
			logger.Warnf("Scheduler received stop signal")
			return

		case <-ctx.Done():
			logger.Warnf("Scheduler context cancelled")
			return
		}
	}
}

func (s *Scheduler) processMessages(ctx context.Context) {
	s.mu.Lock()
	s.lastRunAt = time.Now()
	s.runsCount++
	runNumber := s.runsCount
	failureRate := s.failureRate
	alertWebhook := s.alertWebhook
	alertThreshold := s.alertThreshold
	s.mu.Unlock()

	logger.Infof("[Run #%d] Starting message processing at %s", runNumber, s.lastRunAt.Format(time.RFC3339))

	results, err := s.messageService.ProcessUnsentMessages(ctx, failureRate)
	if err != nil {
		logger.Errorf("[Run #%d] Error processing messages: %v", runNumber, err)
		return
	}

	if results == nil {
		logger.Debugf("[Run #%d] No messages to process", runNumber)
		return
	}

	// Count successful sends
	successCount := 0
	allFailed := true
	for _, r := range results {
		if r.Success {
			successCount++
			allFailed = false
		}
	}

	s.mu.Lock()
	s.messagesSent += int64(successCount)

	// Track consecutive all-fail iterations
	if allFailed && len(results) > 0 {
		s.consecutiveAllFailCount++
		logger.Warnf("[Run #%d] All %d messages failed (consecutive count: %d/%d)",
			runNumber, len(results), s.consecutiveAllFailCount, alertThreshold)

		// Send alert if threshold reached
		if s.consecutiveAllFailCount >= alertThreshold && alertThreshold > 0 && alertWebhook != "" {
			go s.sendAlert(alertWebhook, runNumber, s.consecutiveAllFailCount, len(results))
		}
	} else {
		// Reset counter if any message succeeded
		if s.consecutiveAllFailCount > 0 {
			logger.Debugf(
				"[Run #%d] Resetting consecutive failure count (was: %d)",
				runNumber,
				s.consecutiveAllFailCount,
			)
		}
		s.consecutiveAllFailCount = 0
	}
	s.mu.Unlock()

	logger.Infof("[Run #%d] Processed %d messages, %d successful, %d failed",
		runNumber, len(results), successCount, len(results)-successCount)
}

func (s *Scheduler) Stop() error {
	s.mu.Lock()

	if !s.running {
		s.mu.Unlock()
		logger.Warnf("Scheduler is not running")
		return nil
	}

	s.running = false
	stopChan := s.stopChan
	doneChan := s.doneChan
	s.mu.Unlock()

	// Send stop signal
	close(stopChan)

	// Wait for goroutine to finish
	<-doneChan

	logger.Infof("Scheduler stopped")
	return nil
}

func (s *Scheduler) IsRunning() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.running
}

func (s *Scheduler) GetStatus() SchedulerStatus {
	s.mu.RLock()
	defer s.mu.RUnlock()

	status := SchedulerStatus{
		Running:                 s.running,
		LastRunAt:               s.lastRunAt,
		MessagesSent:            s.messagesSent,
		RunsCount:               s.runsCount,
		Interval:                s.interval,
		ConsecutiveAllFailCount: s.consecutiveAllFailCount,
		LastAlertSentAt:         s.lastAlertSentAt,
	}

	if s.running && !s.lastRunAt.IsZero() {
		status.NextRunAt = s.lastRunAt.Add(s.interval)
	}

	return status
}

func (s *Scheduler) sendAlert(webhookURL string, runNumber int64, consecutiveFailures int, messagesInBatch int) {
	alertPayload := map[string]any{
		"alert":               "consecutive_all_fail",
		"runNumber":           runNumber,
		"consecutiveFailures": consecutiveFailures,
		"messagesInBatch":     messagesInBatch,
		"timestamp":           time.Now().Format(time.RFC3339),
		"message": fmt.Sprintf(
			"All %d messages failed for %d consecutive iterations",
			messagesInBatch,
			consecutiveFailures,
		),
	}

	jsonData, err := json.Marshal(alertPayload)
	if err != nil {
		logger.Errorf("Failed to marshal alert payload: %v", err)
		return
	}

	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		logger.Errorf("Failed to send alert to webhook: %v", err)
		return
	}

	defer func() {
		if err := resp.Body.Close(); err != nil {
			logger.Warnf("Failed to close alert webhook response body: %v", err)
		}
	}()

	if resp.StatusCode == http.StatusOK || resp.StatusCode == http.StatusNoContent {
		s.mu.Lock()
		s.lastAlertSentAt = time.Now()
		s.mu.Unlock()
		logger.Infof("Alert sent successfully to %s (consecutive failures: %d)", webhookURL, consecutiveFailures)
	} else {
		logger.Warnf("Alert webhook returned status %d", resp.StatusCode)
	}
}

type SchedulerStatus struct {
	Running                 bool          `json:"running"`
	LastRunAt               time.Time     `json:"lastRunAt,omitempty"`
	NextRunAt               time.Time     `json:"nextRunAt,omitempty"`
	MessagesSent            int64         `json:"messagesSent"`
	RunsCount               int64         `json:"runsCount"`
	Interval                time.Duration `json:"interval"`
	ConsecutiveAllFailCount int           `json:"consecutiveAllFailCount"`
	LastAlertSentAt         time.Time     `json:"lastAlertSentAt,omitempty"`
}
