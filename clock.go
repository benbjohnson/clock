package clock

import (
	"container/heap"
	"runtime"
	"sync"
	"time"
)

// Clock represents an interface to the functions in the standard library time
// package. Two implementations are available in the clock package. The first
// is a real-time clock which simply wraps the time package's functions. The
// second is a mock clock which will only make forward progress when
// programmatically adjusted.
type Clock interface {
	After(d time.Duration) <-chan time.Time
	AfterFunc(d time.Duration, f func()) *Timer
	Now() time.Time
	Since(t time.Time) time.Duration
	Sleep(d time.Duration)
	Tick(d time.Duration) <-chan time.Time
	Ticker(d time.Duration) *Ticker
	Timer(d time.Duration) *Timer
}

// New returns an instance of a real-time clock.
func New() Clock {
	return &clock{}
}

// clock implements a real-time clock by simply wrapping the time package functions.
type clock struct{}

func (clock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func (clock) AfterFunc(d time.Duration, f func()) *Timer {
	return &Timer{realTimer: time.AfterFunc(d, f)}
}

func (clock) Now() time.Time { return time.Now() }

func (clock) Since(t time.Time) time.Duration { return time.Since(t) }

func (clock) Sleep(d time.Duration) { time.Sleep(d) }

func (clock) Tick(d time.Duration) <-chan time.Time { return time.Tick(d) }

func (clock) Ticker(d time.Duration) *Ticker {
	t := time.NewTicker(d)
	return &Ticker{C: t.C, realTicker: t}
}

func (clock) Timer(d time.Duration) *Timer {
	t := time.NewTimer(d)
	return &Timer{C: t.C, realTimer: t}
}

// Mock represents a mock clock that only moves forward programmically.
// It can be preferable to a real-time clock when testing time-based functionality.
type Mock struct {
	mu     sync.Mutex
	now    time.Time   // current time
	timers clockTimers // tickers & timers
}

// NewMock returns an instance of a mock clock.
// The current time of the mock clock on initialization is the Unix epoch.
func NewMock() *Mock {
	return &Mock{now: time.Unix(0, 0)}
}

// Add moves the current time of the mock clock forward by the duration.
// This is safe for concurrent use.
func (m *Mock) Add(d time.Duration) {
	m.Set(m.Now().Add(d))
}

// Set sets the current time of the mock clock to a specific one.
// This should only be called from a single goroutine at a time.
func (m *Mock) Set(t time.Time) {
	// Continue to execute timers until there are no more before the new time.
	for {
		if timer, tick := m.runNextTimer(t); timer != nil {
			timer.execute(tick)
			runtime.Gosched()
		} else {
			break
		}
	}

	// Ensure that we end with the new time.
	m.mu.Lock()
	if t.After(m.now) {
		m.now = t
	}
	m.mu.Unlock()
}

// runNextTimer returns the next timer in chronological order and moves the
// current time to the timer's next tick time. The next time is not executed if
// its next time if after the max time. Returns true if a timer is executed.
func (m *Mock) runNextTimer(until time.Time) (clockTimer, time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// If we have no more timers then exit.
	if len(m.timers) == 0 {
		return nil, time.Time{}
	}

	// Retrieve next timer. Exit if it fires after `until`.
	t := m.timers[0]
	at := t.tick()

	if at.After(until) {
		return nil, time.Time{}
	}

	// Move "now" forward and remove the timer.
	m.now = at
	if t.singleshot() {
		heap.Pop(&m.timers)
	} else {
		t.update()
		heap.Fix(&m.timers, 0)
	}
	return t, at
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

// Since returns time since the mock clocks wall time.
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
	heap.Push(&m.timers, t)
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
	heap.Push(&m.timers, t)
	return t
}

func (m *Mock) removeClockTimer(t clockTimer) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.removeClockTimerLocked(t)
}

func (m *Mock) removeClockTimerLocked(t clockTimer) {
	for i, timer := range m.timers {
		if timer == t {
			heap.Remove(&m.timers, i)
			return
		}
	}
}

// clockTimer represents an object with an associated "tick" time.
type clockTimer interface {
	tick() time.Time
	execute(time.Time)
	singleshot() bool
	update()
}

// clockTimers represents a heap of timers.
type clockTimers []clockTimer

func (a clockTimers) Len() int           { return len(a) }
func (a clockTimers) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a clockTimers) Less(i, j int) bool { return a[i].tick().Before(a[j].tick()) }
func (a *clockTimers) Push(x interface{}) {
	t := x.(clockTimer)
	(*a) = append(*a, t)
}
func (a *clockTimers) Pop() interface{} {
	old := *a
	n := len(old)
	x := old[n-1]
	*a = old[0 : n-1]
	return x
}

// Timer represents a single event.
// The current time will be sent on C, unless the timer was created by AfterFunc.
type Timer struct {
	C       <-chan time.Time
	c       chan time.Time
	next    time.Time // next tick time
	mock    *Mock     // mock clock, if set
	fn      func()    // AfterFunc function, if set
	stopped bool      // True if stopped, false if running

	// For the real clock impl, only this field is used.
	realTimer *time.Timer
}

// Stop turns off the ticker.
func (t *Timer) Stop() bool {
	if t.realTimer != nil {
		return t.realTimer.Stop()
	}

	t.mock.mu.Lock()
	defer t.mock.mu.Unlock()

	registered := !t.stopped
	t.mock.removeClockTimerLocked(t)
	t.stopped = true
	return registered
}

// Reset changes the expiry time of the timer
func (t *Timer) Reset(d time.Duration) bool {
	if t.realTimer != nil {
		return t.realTimer.Reset(d)
	}

	t.mock.mu.Lock()
	defer t.mock.mu.Unlock()

	registered := !t.stopped
	t.stopped = false
	t.mock.removeClockTimerLocked(t)
	t.next = t.mock.now.Add(d)
	heap.Push(&t.mock.timers, t)

	return registered
}

func (t *Timer) tick() time.Time  { return t.next }
func (t *Timer) singleshot() bool { return true }
func (t *Timer) update()          {}
func (t *Timer) execute(now time.Time) {
	if t.fn != nil {
		t.fn()
	} else {
		t.c <- now
	}

	t.stopped = true
}

// Ticker holds a channel that receives "ticks" at regular intervals.
type Ticker struct {
	C          <-chan time.Time
	c          chan time.Time
	realTicker *time.Ticker  // realtime impl, if set
	next       time.Time     // next tick time
	mock       *Mock         // mock clock, if set
	d          time.Duration // time between ticks
}

// Stop turns off the ticker.
func (t *Ticker) Stop() {
	if t.realTicker != nil {
		t.realTicker.Stop()
	} else {
		t.mock.removeClockTimer(t)
	}
}

func (t *Ticker) tick() time.Time  { return t.next }
func (t *Ticker) singleshot() bool { return false }
func (t *Ticker) update()          { t.next = t.next.Add(t.d) }
func (t *Ticker) execute(now time.Time) {
	select {
	case t.c <- now:
	default:
	}
}
