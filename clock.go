package clock

import (
	"time"
)

// Clock represents an interface to the functions in the standard library time
// package. Two implementations are available in the clock package. The first
// is a real-time clock which simply wraps the time package's functions. The
// second is a mock clock which will only change when
// programmatically adjusted.
type Clock interface {
	After(d time.Duration) <-chan time.Time
	AfterFunc(d time.Duration, f func()) *Timer
	Now() time.Time
	Since(t time.Time) time.Duration
	Sleep(d time.Duration)
	Tick(d time.Duration) <-chan time.Time
	NewTicker(d time.Duration) *Ticker
	NewTimer(d time.Duration) *Timer
	Confirm()
}

// clock implements a real-time clock by simply wrapping the time package functions.
type clock struct{}

var systemClock Clock = New()

func SetSystemClock(clock Clock) {
	systemClock = clock
}

func After(d time.Duration) <-chan time.Time     { return systemClock.After(d) }
func AfterFunc(d time.Duration, f func()) *Timer { return systemClock.AfterFunc(d, f) }
func Now() time.Time                             { return systemClock.Now() }
func Since(t time.Time) time.Duration            { return systemClock.Since(t) }
func Sleep(d time.Duration)                      { systemClock.Sleep(d) }
func Tick(d time.Duration) <-chan time.Time      { return systemClock.Tick(d) }
func NewTicker(d time.Duration) *Ticker          { return systemClock.NewTicker(d) }
func NewTimer(d time.Duration) *Timer            { return systemClock.NewTimer(d) }
func Confirm()                                   { systemClock.Confirm() }

// New returns an instance of a real-time clock.
func New() Clock {
	return &clock{}
}

func (c *clock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func (c *clock) AfterFunc(d time.Duration, f func()) *Timer {
	return &Timer{timer: time.AfterFunc(d, f)}
}

func (c *clock) Now() time.Time { return time.Now() }

func (c *clock) Since(t time.Time) time.Duration { return time.Since(t) }

func (c *clock) Sleep(d time.Duration) { time.Sleep(d) }

func (c *clock) Tick(d time.Duration) <-chan time.Time { return time.Tick(d) }

func (c *clock) NewTicker(d time.Duration) *Ticker {
	t := time.NewTicker(d)
	return &Ticker{C: t.C, ticker: t}
}

func (c *clock) NewTimer(d time.Duration) *Timer {
	t := time.NewTimer(d)
	return &Timer{C: t.C, timer: t}
}

func (c *clock) Confirm() {}
