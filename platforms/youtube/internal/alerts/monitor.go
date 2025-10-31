package alerts

import (
	"context"
	"errors"
	"log"
	"sync"
	"time"
)

// StreamAlert represents the payload sent when a channel goes live.
type StreamAlert struct {
	ChannelID string `json:"channelId"`
	StreamID  string `json:"streamId,omitempty"`
	Status    string `json:"status"`
}

// StreamChecker determines whether a stream is still active.
type StreamChecker interface {
	IsLive(ctx context.Context, channelID, streamID string) (bool, error)
}

// Logger defines the subset of log.Logger used by the monitor.
type Logger interface {
	Printf(format string, v ...any)
}

// Ticker exposes the parts of time.Ticker used by the monitor.
type Ticker interface {
	C() <-chan time.Time
	Stop()
}

// TickerFactory builds a Ticker for the supplied interval.
type TickerFactory func(time.Duration) Ticker

// MonitorConfig controls monitor construction.
type MonitorConfig struct {
	Checker       StreamChecker
	Interval      time.Duration
	Logger        Logger
	RootContext   context.Context
	TickerFactory TickerFactory
}

// ErrMissingChannelID indicates an alert was missing its channel identifier.
var ErrMissingChannelID = errors.New("alerts: missing channel ID")

// Monitor watches active live streams and polls the YouTube API until a stream ends.
type Monitor struct {
	checker       StreamChecker
	interval      time.Duration
	logger        Logger
	rootCtx       context.Context
	tickerFactory TickerFactory

	mu       sync.Mutex
	watchers map[string]context.CancelFunc
}

// DefaultTickerFactory creates a ticker backed by time.NewTicker.
func DefaultTickerFactory(interval time.Duration) Ticker {
	return &timeTicker{Ticker: time.NewTicker(interval)}
}

type timeTicker struct {
	*time.Ticker
}

func (t *timeTicker) C() <-chan time.Time {
	return t.Ticker.C
}

// NewMonitor builds a new Monitor using the provided configuration.
func NewMonitor(cfg MonitorConfig) *Monitor {
	logger := cfg.Logger
	if logger == nil {
		logger = log.Default()
	}

	tickerFactory := cfg.TickerFactory
	if tickerFactory == nil {
		tickerFactory = DefaultTickerFactory
	}

	rootCtx := cfg.RootContext
	if rootCtx == nil {
		rootCtx = context.Background()
	}

	return &Monitor{
		checker:       cfg.Checker,
		interval:      cfg.Interval,
		logger:        logger,
		rootCtx:       rootCtx,
		tickerFactory: tickerFactory,
		watchers:      make(map[string]context.CancelFunc),
	}
}

// Handle registers a stream alert and starts polling until the stream ends.
func (m *Monitor) Handle(ctx context.Context, alert StreamAlert) error {
	if alert.ChannelID == "" {
		return ErrMissingChannelID
	}

	if alert.Status != "online" {
		m.cancelWatcher(alert.ChannelID)
		m.logger.Printf("status for %s updated to %s; stopping monitor", alert.ChannelID, alert.Status)
		return nil
	}

	m.mu.Lock()
	if _, exists := m.watchers[alert.ChannelID]; exists {
		m.mu.Unlock()
		m.logger.Printf("already monitoring channel %s", alert.ChannelID)
		return nil
	}

	parent := ctx
	if parent == nil {
		parent = m.rootCtx
	}

	watchCtx, cancel := context.WithCancel(parent)
	m.watchers[alert.ChannelID] = cancel
	m.mu.Unlock()

	go m.watch(watchCtx, alert)
	m.logger.Printf("began monitoring channel %s", alert.ChannelID)
	return nil
}

// ActiveWatchers returns the number of active monitors. Primarily used for testing.
func (m *Monitor) ActiveWatchers() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.watchers)
}

// StopAll cancels any active stream monitors.
func (m *Monitor) StopAll() {
	m.mu.Lock()
	defer m.mu.Unlock()

	for channelID, cancel := range m.watchers {
		cancel()
		delete(m.watchers, channelID)
	}
}

func (m *Monitor) cancelWatcher(channelID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cancel, ok := m.watchers[channelID]; ok {
		cancel()
		delete(m.watchers, channelID)
	}
}

func (m *Monitor) watch(ctx context.Context, alert StreamAlert) {
	ticker := m.tickerFactory(m.interval)
	defer ticker.Stop()

	// Perform an immediate status check before waiting for the first tick.
	live, err := m.checker.IsLive(ctx, alert.ChannelID, alert.StreamID)
	if err != nil {
		m.logger.Printf("live check for %s failed: %v", alert.ChannelID, err)
	}
	if !live {
		m.logger.Printf("channel %s is no longer live; stopping monitor", alert.ChannelID)
		m.cancelWatcher(alert.ChannelID)
		return
	}
	m.logger.Printf("channel %s confirmed live", alert.ChannelID)

	for {
		select {
		case <-ctx.Done():
			m.logger.Printf("monitor for %s cancelled", alert.ChannelID)
			return
		case <-ticker.C():
			live, err := m.checker.IsLive(ctx, alert.ChannelID, alert.StreamID)
			if err != nil {
				m.logger.Printf("live check for %s failed: %v", alert.ChannelID, err)
				continue
			}
			if !live {
				m.logger.Printf("channel %s is no longer live; stopping monitor", alert.ChannelID)
				m.cancelWatcher(alert.ChannelID)
				return
			}
			m.logger.Printf("channel %s still live", alert.ChannelID)
		}
	}
}
