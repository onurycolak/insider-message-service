package scheduler

import (
	"context"
	"testing"
	"time"

	"github.com/onurcolak/insider-message-service/internal/domain"
)

// fakeProcessor is a simple test double for messageProcessor.
type fakeProcessor struct {
	resultsToReturn []domain.SendResult
	errToReturn     error

	calls []processCall
}

type processCall struct {
	FailureRate float64
}

func (f *fakeProcessor) ProcessUnsentMessages(ctx context.Context, failureRate float64) ([]domain.SendResult, error) {
	f.calls = append(f.calls, processCall{FailureRate: failureRate})
	return f.resultsToReturn, f.errToReturn
}

func TestScheduler_ProcessMessages_MixedResults(t *testing.T) {
	ctx := context.Background()

	processor := &fakeProcessor{
		resultsToReturn: []domain.SendResult{
			{Success: true},
			{Success: false},
			{Success: true},
		},
	}
	s := &Scheduler{
		messageService: processor,
		interval:       time.Minute,
	}

	// Set some alert config but keep alertWebhook empty so no HTTP calls
	s.alertThreshold = 3
	s.alertWebhook = ""

	s.processMessages(ctx)

	status := s.GetStatus()
	if status.MessagesSent != 2 {
		t.Errorf("expected MessagesSent=2, got %d", status.MessagesSent)
	}
	if status.RunsCount != 1 {
		t.Errorf("expected RunsCount=1, got %d", status.RunsCount)
	}
	if status.ConsecutiveAllFailCount != 0 {
		t.Errorf("expected ConsecutiveAllFailCount=0, got %d", status.ConsecutiveAllFailCount)
	}
	if len(processor.calls) != 1 {
		t.Fatalf("expected 1 call to ProcessUnsentMessages, got %d", len(processor.calls))
	}
}

func TestScheduler_ProcessMessages_AllFailIncrementsCounter(t *testing.T) {
	ctx := context.Background()

	processor := &fakeProcessor{
		resultsToReturn: []domain.SendResult{
			{Success: false},
			{Success: false},
		},
	}
	s := &Scheduler{
		messageService: processor,
		interval:       time.Minute,
		alertThreshold: 5,  // high enough so sendAlert is not triggered
		alertWebhook:   "", // also prevents HTTP calls
	}

	s.processMessages(ctx)

	status := s.GetStatus()
	if status.MessagesSent != 0 {
		t.Errorf("expected MessagesSent=0, got %d", status.MessagesSent)
	}
	if status.RunsCount != 1 {
		t.Errorf("expected RunsCount=1, got %d", status.RunsCount)
	}
	if status.ConsecutiveAllFailCount != 1 {
		t.Errorf("expected ConsecutiveAllFailCount=1, got %d", status.ConsecutiveAllFailCount)
	}
}

func TestScheduler_StartAndStopToggleRunning(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	processor := &fakeProcessor{}
	s := &Scheduler{
		messageService: processor,
		interval:       10 * time.Millisecond,
	}

	if s.IsRunning() {
		t.Fatalf("expected scheduler to be not running initially")
	}

	if err := s.Start(ctx); err != nil {
		t.Fatalf("Start returned error: %v", err)
	}

	if !s.IsRunning() {
		t.Fatalf("expected scheduler to be running after Start")
	}

	if err := s.Stop(); err != nil {
		t.Fatalf("Stop returned error: %v", err)
	}

	if s.IsRunning() {
		t.Fatalf("expected scheduler to be not running after Stop")
	}
}
