package clock

import (
	"sort"
	"sync"
	"testing"
	"time"
)

var (
	WaitForStarts                 = &WaitForStartsOption{}
	IgnoreUnexpectedUpcomingEvent = &IgnoreUnexpectedUpcomingEventOption{}
	OptimisticSched               = &OptimisticSchedOption{}
)

type Confirmable interface {
	Confirm()
}

type Option interface {
	PerformOptionBeforeCall(*Mock)
	PerformOptionAfterCall(*Mock)
}

type FailOnUnexpectedUpcomingEventOption struct {
	t *testing.T
}

func FailOnUnexpectedUpcomingEvent(t *testing.T) *FailOnUnexpectedUpcomingEventOption {
	return &FailOnUnexpectedUpcomingEventOption{t}
}

func (o *FailOnUnexpectedUpcomingEventOption) PerformOptionBeforeCall(mock *Mock) {
}

func (o *FailOnUnexpectedUpcomingEventOption) PerformOptionAfterCall(mock *Mock) {
	mock.tForFail = o.t
}

type IgnoreUnexpectedUpcomingEventOption struct{}

func (o *IgnoreUnexpectedUpcomingEventOption) PerformOptionBeforeCall(mock *Mock) {
}

func (o *IgnoreUnexpectedUpcomingEventOption) PerformOptionAfterCall(mock *Mock) {
	mock.tForFail = nil
}

type ExpectStartsBeforeNextOption struct {
	starts int
}

func ExpectStartsBeforeNext(starts int) *ExpectStartsBeforeNextOption {
	return &ExpectStartsBeforeNextOption{starts}
}

func (o *ExpectStartsBeforeNextOption) PerformOptionBeforeCall(mock *Mock) {
}

func (o *ExpectStartsBeforeNextOption) PerformOptionAfterCall(mock *Mock) {
	mock.ExpectStarts(int(o.starts))
}

type ExpectConfirmsBeforeNextOption struct {
	confirms int
}

func ExpectConfirmsBeforeNext(confirms int) *ExpectConfirmsBeforeNextOption {
	return &ExpectConfirmsBeforeNextOption{confirms}
}

func (o *ExpectConfirmsBeforeNextOption) PerformOptionBeforeCall(mock *Mock) {
}

func (o *ExpectConfirmsBeforeNextOption) PerformOptionAfterCall(mock *Mock) {
	mock.ExpectConfirms(int(o.confirms))
}

type WaitForStartsOption struct{}

func (o *WaitForStartsOption) PerformOptionBeforeCall(mock *Mock) {
	mock.WaitForStart()
}

func (o *WaitForStartsOption) PerformOptionAfterCall(mock *Mock) {
}

type OptimisticSchedOption struct{}

func (o *OptimisticSchedOption) PerformOptionBeforeCall(mock *Mock) {}

func (o *OptimisticSchedOption) PerformOptionAfterCall(mock *Mock) {
	gosched()
}

// Mock represents a mock clock that only moves forward programmatically.
// It can be preferable to a real-time clock when testing time-based functionality.
type Mock struct {
	mu     sync.Mutex
	now    time.Time   // current time
	timers clockTimers // tickers & timers

	newTimers       sync.WaitGroup
	recentTimers    int
	expectingStarts int

	confirms          sync.WaitGroup
	recentConfirms    int
	expectingConfirms int

	tForFail *testing.T
}

// NewMock returns an instance of a mock clock.
// The current time of the mock clock on initialization is the Unix epoch.
func NewMock(opts ...Option) *Mock {
	ret := &Mock{now: time.Unix(0, 0)}
	for _, opt := range opts {
		opt.PerformOptionAfterCall(ret)
	}
	return ret
}

// ExpectStarts informs the mock how many timers should have been created before we advance the clock
func (m *Mock) ExpectStarts(timerCount int) {
	m.mu.Lock()
	m.expectingStarts++
	m.newTimers.Add(timerCount - m.recentTimers)
	m.recentTimers = 0
	m.mu.Unlock()
}

// WaitForStart will block until all expected timers have started
func (m *Mock) WaitForStart() {
	m.newTimers.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expectingStarts = 0
}

// ExpectConfirms informs the mock how many timers should have been confirmed before we advance the clock
func (m *Mock) ExpectConfirms(confirmCount int) {
	m.mu.Lock()
	m.expectingConfirms++
	m.newTimers.Add(confirmCount - m.recentConfirms)
	m.recentConfirms = 0
	m.mu.Unlock()
}

// WaitForConfirm will block until all expected timers have been confirmed
func (m *Mock) WaitForConfirm() {
	m.confirms.Wait()
	m.mu.Lock()
	defer m.mu.Unlock()
	m.expectingConfirms = 0
}

// Add moves the current time of the mock clock forward by the specified duration.
// This should only be called from a single goroutine at a time.
func (m *Mock) Add(d time.Duration, opts ...Option) {
	for _, opt := range opts {
		opt.PerformOptionBeforeCall(m)
	}
	// Calculate the final current time.
	t := m.now.Add(d)

	// Continue to execute timers until there are no more before the new time.
	for {
		if !m.runNextTimer(t) {
			break
		}
	}

	// Ensure that we end with the new time.
	m.mu.Lock()
	m.now = t
	m.mu.Unlock()

	for _, opt := range opts {
		opt.PerformOptionAfterCall(m)
	}
}

// Set sets the current time of the mock clock to a specific one.
// This should only be called from a single goroutine at a time.
func (m *Mock) Set(t time.Time, opts ...Option) {
	for _, opt := range opts {
		opt.PerformOptionBeforeCall(m)
	}
	// Continue to execute timers until there are no more before the new time.
	for {
		if !m.runNextTimer(t) {
			break
		}
	}

	// Ensure that we end with the new time.
	m.mu.Lock()
	m.now = t
	m.mu.Unlock()

	for _, opt := range opts {
		opt.PerformOptionAfterCall(m)
	}
}

// runNextTimer executes the next timer in chronological order and moves the
// current time to the timer's next tick time. The next time is not executed if
// its next time is after the max time. Returns true if a timer was executed.
func (m *Mock) runNextTimer(max time.Time) bool {
	m.mu.Lock()

	// Sort timers by time.
	sort.Sort(m.timers)

	// If we have no more timers then exit.
	if len(m.timers) == 0 {
		m.mu.Unlock()
		return false
	}

	// Retrieve next timer. Exit if next tick is after new time.
	t := m.timers[0]
	if t.Next().After(max) {
		m.mu.Unlock()
		return false
	}

	// Move "now" forward and unlock clock.
	m.now = t.Next()
	m.mu.Unlock()

	// Execute timer.
	t.Tick(m.now)
	return true
}

// After waits for the duration to elapse and then sends the current time on the returned channel.
func (m *Mock) After(d time.Duration) <-chan time.Time {
	return m.Timer(d).C
}

// AfterFunc waits for the duration to elapse and then executes a function.
// A Timer is returned that can be stopped.
func (m *Mock) AfterFunc(d time.Duration, f func()) *Timer {
	t := m.Timer(d)
	t.C = nil
	t.fn = f
	return t
}

// Now returns the current wall time on the mock clock.
func (m *Mock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// Since returns time since the mock clock's wall time.
func (m *Mock) Since(t time.Time) time.Duration {
	return m.Now().Sub(t)
}

// Sleep pauses the goroutine for the given duration on the mock clock.
// The clock must be moved forward in a separate goroutine.
func (m *Mock) Sleep(d time.Duration) {
	<-m.After(d)
}

// Tick is a convenience function for Ticker().
// It will return a ticker channel that cannot be stopped.
func (m *Mock) Tick(d time.Duration) <-chan time.Time {
	return m.Ticker(d).C
}

// Ticker creates a new instance of Ticker.
func (m *Mock) Ticker(d time.Duration) *Ticker {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan time.Time, 1)
	t := &Ticker{
		C:    ch,
		c:    ch,
		mock: m,
		d:    d,
		next: m.now.Add(d),
	}
	m.timers = append(m.timers, (*internalTicker)(t))
	m.recentTimers++
	if m.expectingStarts > 0 {
		m.newTimers.Done() // signal that we started a timer
	} else if m.tForFail != nil {
		m.tForFail.Errorf("unexpected ticker start")
	} else {
		m.recentTimers++
	}
	return t
}

// Timer creates a new instance of Timer.
func (m *Mock) Timer(d time.Duration) *Timer {
	m.mu.Lock()
	defer m.mu.Unlock()
	ch := make(chan time.Time, 1)
	t := &Timer{
		C:       ch,
		c:       ch,
		mock:    m,
		next:    m.now.Add(d),
		stopped: false,
	}
	m.timers = append(m.timers, (*internalTimer)(t))
	if m.expectingStarts > 0 {
		m.newTimers.Done() // signal that we started a timer
	} else if m.tForFail != nil {
		m.tForFail.Errorf("unexpected timer start")
	} else {
		m.recentTimers++
	}
	return t
}

func (m *Mock) removeClockTimer(t clockTimer) {
	for i, timer := range m.timers {
		if timer == t {
			copy(m.timers[i:], m.timers[i+1:])
			m.timers[len(m.timers)-1] = nil
			m.timers = m.timers[:len(m.timers)-1]
			break
		}
	}
	sort.Sort(m.timers)
}

type internalTimer Timer

func (t *internalTimer) Next() time.Time { return t.next }
func (t *internalTimer) Tick(now time.Time) {
	t.mock.mu.Lock()
	if t.fn != nil {
		t.fn()
	} else {
		t.c <- now
	}
	t.mock.removeClockTimer((*internalTimer)(t))
	t.stopped = true
	t.mock.mu.Unlock()
	gosched()
}

type internalTicker Ticker

func (t *internalTicker) Next() time.Time { return t.next }
func (t *internalTicker) Tick(now time.Time) {
	select {
	case t.c <- now:
	default:
	}
	t.next = now.Add(t.d)
	gosched()
}

// Sleep momentarily so that other goroutines can process.
func gosched() { time.Sleep(1 * time.Millisecond) }
