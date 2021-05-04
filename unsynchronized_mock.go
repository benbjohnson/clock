package clock

import (
	"sort"
	"sync"
	"testing"
	"time"
)

var (
	WaitForStartsBefore           = &WaitForStartsBeforeOption{}
	WaitForConfirmsBefore         = &WaitForConfirmsBeforeOption{}
	WaitBefore                    = &WaitBeforeOption{}
	WaitForStartsAfter            = &WaitForStartsAfterOption{}
	WaitForConfirmsAfter          = &WaitForConfirmsAfterOption{}
	WaitAfter                     = &WaitAfterOption{}
	IgnoreUnexpectedUpcomingEvent = &IgnoreUnexpectedUpcomingEventOption{}
	OptimisticSched               = &OptimisticSchedOption{}
)

type Option interface {
	PriorEventsOption(*UnsynchronizedMock)
	UpcomingEventsOption(*UnsynchronizedMock)
	AfterClockAdvanceOption(*UnsynchronizedMock)
}

type FailOnUnexpectedUpcomingEventOption struct {
	t *testing.T
}

func FailOnUnexpectedUpcomingEvent(t *testing.T) *FailOnUnexpectedUpcomingEventOption {
	return &FailOnUnexpectedUpcomingEventOption{t}
}

func (o *FailOnUnexpectedUpcomingEventOption) PriorEventsOption(mock *UnsynchronizedMock) {}

func (o *FailOnUnexpectedUpcomingEventOption) UpcomingEventsOption(mock *UnsynchronizedMock) {
	mock.tForFail = o.t
}

func (o *FailOnUnexpectedUpcomingEventOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {}

type IgnoreUnexpectedUpcomingEventOption struct{}

func (o *IgnoreUnexpectedUpcomingEventOption) PriorEventsOption(mock *UnsynchronizedMock) {}

func (o *IgnoreUnexpectedUpcomingEventOption) UpcomingEventsOption(mock *UnsynchronizedMock) {
	mock.tForFail = nil
}

func (o *IgnoreUnexpectedUpcomingEventOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {}

type ExpectUpcomingStartsOption struct {
	starts int
}

func ExpectUpcomingStarts(starts int) *ExpectUpcomingStartsOption {
	return &ExpectUpcomingStartsOption{starts}
}

func (o *ExpectUpcomingStartsOption) PriorEventsOption(mock *UnsynchronizedMock) {}

func (o *ExpectUpcomingStartsOption) UpcomingEventsOption(mock *UnsynchronizedMock) {
	mock.ExpectStarts(int(o.starts))
}

func (o *ExpectUpcomingStartsOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {}

type ExpectUpcomingConfirmsOption struct {
	confirms int
}

func ExpectUpcomingConfirms(confirms int) *ExpectUpcomingConfirmsOption {
	return &ExpectUpcomingConfirmsOption{confirms}
}

func (o *ExpectUpcomingConfirmsOption) PriorEventsOption(mock *UnsynchronizedMock) {}

func (o *ExpectUpcomingConfirmsOption) UpcomingEventsOption(mock *UnsynchronizedMock) {
	mock.ExpectConfirms(int(o.confirms))
}

func (o *ExpectUpcomingConfirmsOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {}

type WaitForStartsBeforeOption struct{}

func (o *WaitForStartsBeforeOption) PriorEventsOption(mock *UnsynchronizedMock) {
	mock.WaitForStart()
}

func (o *WaitForStartsBeforeOption) UpcomingEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitForStartsBeforeOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {}

type WaitForConfirmsBeforeOption struct{}

func (o *WaitForConfirmsBeforeOption) PriorEventsOption(mock *UnsynchronizedMock) {
	mock.WaitForConfirm()
}

func (o *WaitForConfirmsBeforeOption) UpcomingEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitForConfirmsBeforeOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {}

type WaitBeforeOption struct{}

func (o *WaitBeforeOption) PriorEventsOption(mock *UnsynchronizedMock) {
	mock.Wait()
}

func (o *WaitBeforeOption) UpcomingEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitBeforeOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {}

type WaitForStartsAfterOption struct{}

func (o *WaitForStartsAfterOption) PriorEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitForStartsAfterOption) UpcomingEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitForStartsAfterOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {
	mock.WaitForStart()
}

type WaitForConfirmsAfterOption struct{}

func (o *WaitForConfirmsAfterOption) PriorEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitForConfirmsAfterOption) UpcomingEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitForConfirmsAfterOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {
	mock.WaitForConfirm()
}

type WaitAfterOption struct{}

func (o *WaitAfterOption) PriorEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitAfterOption) UpcomingEventsOption(mock *UnsynchronizedMock) {}

func (o *WaitAfterOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {
	mock.Wait()
}

type OptimisticSchedOption struct{}

func (o *OptimisticSchedOption) PriorEventsOption(mock *UnsynchronizedMock) {}

func (o *OptimisticSchedOption) UpcomingEventsOption(mock *UnsynchronizedMock) {
	gosched()
}

func (o *OptimisticSchedOption) AfterClockAdvanceOption(mock *UnsynchronizedMock) {}

// UnsynchronizedMock represents a mock clock that only moves forward programmatically.
// It can be preferable to a real-time clock when testing time-based functionality. By
// default, it does not enforce synchronization although options may be passed in to
// cause sync.
type UnsynchronizedMock struct {
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

// NewUnsynchronizedMock returns an instance of a mock clock.
// The current time of the mock clock on initialization is the Unix epoch.
func NewUnsynchronizedMock(opts ...Option) *UnsynchronizedMock {
	ret := &UnsynchronizedMock{now: time.Unix(0, 0)}
	for _, opt := range opts {
		opt.UpcomingEventsOption(ret)
	}
	return ret
}

// ExpectStarts informs the mock how many timers should have been created before we advance the clock
func (m *UnsynchronizedMock) ExpectStarts(timerCount int) {
	m.mu.Lock()
	newStarts := timerCount - m.recentTimers
	m.expectingStarts = m.expectingStarts + newStarts
	m.newTimers.Add(newStarts)
	m.recentTimers = 0
	m.mu.Unlock()
}

// WaitForStart will block until all expected timers have started
func (m *UnsynchronizedMock) WaitForStart() {
	m.newTimers.Wait()
}

// ExpectConfirms informs the mock how many timers should have been confirmed before we advance the clock
func (m *UnsynchronizedMock) ExpectConfirms(confirmCount int) {
	m.mu.Lock()
	newconfirms := confirmCount - m.recentConfirms
	m.expectingConfirms = m.expectingConfirms + newconfirms
	m.confirms.Add(newconfirms)
	m.recentConfirms = 0
	m.mu.Unlock()
}

// WaitForConfirm will block until all expected timers have been confirmed
func (m *UnsynchronizedMock) WaitForConfirm() {
	m.confirms.Wait()
}

// Wait waits for starts and confirms before returning
func (m *UnsynchronizedMock) Wait() {
	m.WaitForStart()
	m.WaitForConfirm()
}

// Add moves the current time of the mock clock forward by the specified duration.
// This should only be called from a single goroutine at a time.
func (m *UnsynchronizedMock) Add(d time.Duration, opts ...Option) {
	for _, opt := range opts {
		opt.PriorEventsOption(m)
	}

	for _, opt := range opts {
		opt.UpcomingEventsOption(m)
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
}

// Set sets the current time of the mock clock to a specific one.
// This should only be called from a single goroutine at a time.
func (m *UnsynchronizedMock) Set(t time.Time, opts ...Option) {
	for _, opt := range opts {
		opt.PriorEventsOption(m)
	}

	for _, opt := range opts {
		opt.UpcomingEventsOption(m)
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
}

// runNextTimer executes the next timer in chronological order and moves the
// current time to the timer's next tick time. The next time is not executed if
// its next time is after the max time. Returns true if a timer was executed.
func (m *UnsynchronizedMock) runNextTimer(max time.Time) bool {
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
func (m *UnsynchronizedMock) After(d time.Duration) <-chan time.Time {
	return m.NewTimer(d).C
}

// AfterFunc waits for the duration to elapse and then executes a function.
// A Timer is returned that can be stopped.
func (m *UnsynchronizedMock) AfterFunc(d time.Duration, f func()) *Timer {
	t := m.NewTimer(d)
	t.C = nil
	t.fn = f
	return t
}

// Now returns the current wall time on the mock clock.
func (m *UnsynchronizedMock) Now() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// Since returns time since the mock clock's wall time.
func (m *UnsynchronizedMock) Since(t time.Time) time.Duration {
	return m.Now().Sub(t)
}

// Sleep pauses the goroutine for the given duration on the mock clock.
// The clock must be moved forward in a separate goroutine.
func (m *UnsynchronizedMock) Sleep(d time.Duration) {
	<-m.After(d)
}

// Tick is a convenience function for Ticker().
// It will return a ticker channel that cannot be stopped.
func (m *UnsynchronizedMock) Tick(d time.Duration) <-chan time.Time {
	return m.NewTicker(d).C
}

// NewTicker creates a new instance of NewTicker.
func (m *UnsynchronizedMock) NewTicker(d time.Duration) *Ticker {
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
		m.expectingStarts--
		m.newTimers.Done() // signal that we started a timer
	} else if m.tForFail != nil {
		m.tForFail.Errorf("unexpected ticker start")
	} else {
		m.recentTimers++
	}
	return t
}

// NewTimer creates a new instance of NewTimer.
func (m *UnsynchronizedMock) NewTimer(d time.Duration) *Timer {
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
		m.expectingStarts--
		m.newTimers.Done() // signal that we started a timer
	} else if m.tForFail != nil {
		m.tForFail.Errorf("unexpected timer start")
	} else {
		m.recentTimers++
	}
	return t
}

func (m *UnsynchronizedMock) Confirm() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.expectingConfirms > 0 {
		m.expectingConfirms--
		m.confirms.Done() // signal that timer has been confirmed
	} else if m.tForFail != nil {
		m.tForFail.Helper()
		m.tForFail.Errorf("Unexpected ticker confirmation")
	} else {
		m.recentConfirms++
	}
}

func (m *UnsynchronizedMock) removeClockTimer(t clockTimer) {
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
		t.mock.mu.Unlock()
		t.fn()
		t.mock.mu.Lock()
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
