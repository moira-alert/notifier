package notifier

import (
	"sync"
	"time"

	"github.com/gosexy/to"
	"github.com/rcrowley/go-metrics"
)

var (
	wg                     sync.WaitGroup
	eventsReceived         = metrics.NewRegisteredMeter("events.received", metrics.DefaultRegistry)
	eventsMalformed        = metrics.NewRegisteredMeter("events.malformed", metrics.DefaultRegistry)
	eventsProcessingFailed = metrics.NewRegisteredMeter("events.failed", metrics.DefaultRegistry)
	subsMalformed          = metrics.NewRegisteredMeter("subs.malformed", metrics.DefaultRegistry)
	sendingFailed          = metrics.NewRegisteredMeter("sending.failed", metrics.DefaultRegistry)
	senderTimeout          time.Duration
	resendingTimeout       time.Duration
	sending                = make(map[string]chan notificationPackage)
	sendersOkMetrics       = make(map[string]metrics.Meter)
	sendersFailedMetrics   = make(map[string]metrics.Meter)

	log    Logger
	db     Database
	config *Config

	// GetNow allows you to travel in time while testing
	GetNow = func() time.Time {
		return time.Now()
	}
)

// GetWaitGroup return senders wait group
func GetWaitGroup() *sync.WaitGroup {
	return &wg
}

// SetLogger allows you to redefine logging in tests
func SetLogger(logger Logger) {
	log = logger
}

// SetDb allows you to use mock database in tests
func SetDb(connector Database) {
	db = connector
}

// SetSettings allows you to redefine config in tests
func SetSettings(c *Config) {
	config = c
	senderTimeout = to.Duration(config.Notifier.SenderTimeout)
	resendingTimeout = to.Duration(config.Notifier.ResendingTimeout)
}
