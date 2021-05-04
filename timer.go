package clock

import "time"

// clockTimer represents an object with an associated start time.
type clockTimer interface {
	Next() time.Time
	Tick(time.Time)
}

// clockTimers represents a list of sortable timers.
type clockTimers []clockTimer

func (a clockTimers) Len() int           { return len(a) }
func (a clockTimers) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a clockTimers) Less(i, j int) bool { return a[i].Next().Before(a[j].Next()) }

// Timer represents a single event.
// The current time will be sent on C, unless the timer was created by AfterFunc.
type Timer struct {
	C       <-chan time.Time
	c       chan time.Time
	timer   *time.Timer         // realtime impl, if set
	next    time.Time           // next tick time
	mock    *UnsynchronizedMock // mock clock, if set
	fn      func()              // AfterFunc function, if set
	stopped bool                // True if stopped, false if running
}

// Stop turns off the ticker.
func (t *Timer) Stop() bool {
	if t.timer != nil {
		return t.timer.Stop()
	}

	t.mock.mu.Lock()
	registered := !t.stopped
	t.mock.removeClockTimer((*internalTimer)(t))
	t.stopped = true
	t.mock.mu.Unlock()
	return registered
}

// Reset changes the expiry time of the timer
func (t *Timer) Reset(d time.Duration) bool {
	if t.timer != nil {
		return t.timer.Reset(d)
	}

	t.mock.mu.Lock()
	t.next = t.mock.now.Add(d)
	defer t.mock.mu.Unlock()

	registered := !t.stopped
	if t.stopped {
		t.mock.timers = append(t.mock.timers, (*internalTimer)(t))
	}

	t.stopped = false
	return registered
}

// Confirm confirms that a timer event has been processed - no op for system clock, but allows synchronization of the mock
func (t *Timer) Confirm() {
	if t.timer != nil {
		return
	}

	t.mock.Confirm()
}

// Ticker holds a channel that receives "ticks" at regular intervals.
type Ticker struct {
	C      <-chan time.Time
	c      chan time.Time
	ticker *time.Ticker        // realtime impl, if set
	next   time.Time           // next tick time
	mock   *UnsynchronizedMock // mock clock, if set
	d      time.Duration       // time between ticks
}

// Stop turns off the ticker.
func (t *Ticker) Stop() {
	if t.ticker != nil {
		t.ticker.Stop()
	} else {
		t.mock.mu.Lock()
		t.mock.removeClockTimer((*internalTicker)(t))
		t.mock.mu.Unlock()
	}
}

// Reset resets the ticker to a new duration.
func (t *Ticker) Reset(dur time.Duration) {
	if t.ticker != nil {
		t.ticker.Reset(dur)
		return
	}

	t.mock.mu.Lock()
	defer t.mock.mu.Unlock()

	t.d = dur
	t.next = t.mock.now.Add(dur)
}

// Confirm confirms that a ticker event has been processed - no op for system clock, but allows synchronization of the mock
func (t *Ticker) Confirm() {
	if t.ticker != nil {
		return
	}

	t.mock.Confirm()
}
