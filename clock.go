// Copyright (c) 2014 Ben Johnson

package clock

import (
	"fmt"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"sync"
	"time"
)

var skipThisPackageRegExp = regexp.MustCompile(".*/aristanetworks/clock(/|/_test/_obj_test/)clock.go")

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

func (c *clock) After(d time.Duration) <-chan time.Time { return time.After(d) }

func (c *clock) AfterFunc(d time.Duration, f func()) *Timer {
	return &Timer{timer: time.AfterFunc(d, f)}
}

func (c *clock) Now() time.Time { return time.Now() }

func (c *clock) Since(t time.Time) time.Duration { return time.Since(t) }

func (c *clock) Sleep(d time.Duration) { time.Sleep(d) }

func (c *clock) Tick(d time.Duration) <-chan time.Time { return time.Tick(d) }

func (c *clock) Ticker(d time.Duration) *Ticker {
	t := time.NewTicker(d)
	return &Ticker{C: t.C, ticker: t}
}

func (c *clock) Timer(d time.Duration) *Timer {
	t := time.NewTimer(d)
	return &Timer{C: t.C, timer: t}
}

// Mock is an interface returned by NewMock that gives access to the
// mocked clock to be provided to the tested code and the ability to control the
// flow of time.
type Mock interface {
	Add(d time.Duration)
	Set(t time.Time)
	Now() time.Time
	Clock() Clock
	NextTimer() (time.Time, error)
	SetHook(point HookPoint, h Hook)
	SetHookExt(point HookPoint, fn string, file string, h Hook)
	ResetHooks()
	DelHooks(point HookPoint, fn string, file string)
}

type mock struct {
	*mockClock
}

func (m *mock) Clock() Clock {
	return m.mockClock
}

// Now is to read Now without any side effects by the test
func (m *mock) Now() time.Time {
	return m.nowInternal()
}

// HookPoint defines where the hook should be placed
type HookPoint int

const (
	// HookBeforeNow is called just before we read the current time
	HookBeforeNow HookPoint = iota
	// HookAfterNow is called just after we read the current time
	HookAfterNow
	// HookBeforeTimer is called just before we set a timer
	HookBeforeTimer
	// HookAfterTimer is called just after we set a timer
	HookAfterTimer
)

// hookCall specifies when exactly a hook should be called, that means, it is
// called as a given HookPoint, but only if called from fn and/or from the file.
// Both is treated as a suffix
type hookCall struct {
	fn   *string
	file *string
	h    Hook
}

func (c *hookCall) String() string {
	fn := "*"
	file := "*"
	if c.fn != nil {
		fn = *c.fn
	}
	if c.file != nil {
		file = *c.file
	}
	return fmt.Sprintf("fn : %s file %s hook %+v", fn, file, c.h)
}

// Hook is called with the current mock time and returns the new time mock
// should set. If the time is the same, nothing changes
type Hook func(time.Time)

// mockClock represents a mock clock that only moves forward programmically.
type mockClock struct {
	mu     sync.Mutex
	now    time.Time   // current time
	timers clockTimers // tickers & timers

	hooks map[HookPoint][]*hookCall
}

// NewMock returns an instance of Mock that allows the test to control the flow
// of time in the tested code
func NewMock() Mock {
	return &mock{
		mockClock: &mockClock{
			now:   time.Unix(0, 0),
			hooks: make(map[HookPoint][]*hookCall),
		},
	}
}

// Add moves the current time of the mock clock forward by the duration.
// This should only be called from a single goroutine at a time.
func (m *mockClock) Add(d time.Duration) {
	if d < 0 {
		panic(fmt.Sprintf("Adding negative duration %s", d))
	}

	// Calculate the final current time.
	m.mu.Lock()
	t := m.now.Add(d)
	m.mu.Unlock()

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
func (m *mockClock) Set(t time.Time) {

	if now := m.nowInternal(); now.After(t) {
		panic(fmt.Sprintf("Setting time %s in the past (now %s)", t, now))
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
// it's next time if after the max time. Returns true if a timer is executed.
func (m *mockClock) runNextTimer(max time.Time) bool {
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
	now := t.Next()
	m.now = now
	m.mu.Unlock()

	// Execute timer.
	t.Tick(now)
	return true
}

// NextTimer returns when the next timer is scheduled or an error if there
// is not timer pending
func (m *mockClock) NextTimer() (time.Time, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.timers) == 0 {
		return time.Unix(0, 0), fmt.Errorf("No timers")
	}

	// Sort timers by time.
	sort.Sort(m.timers)
	return m.timers[0].Next(), nil
}

// After waits for the duration to elapse and then sends the current time on the returned channel.
func (m *mockClock) After(d time.Duration) <-chan time.Time {
	return m.timerInternal(d).C
}

// AfterFunc waits for the duration to elapse and then executes a function.
// A Timer is returned that can be stopped.
func (m *mockClock) AfterFunc(d time.Duration, f func()) *Timer {
	t := m.Timer(d)
	t.C = nil
	t.fn = f
	return t
}

func (m *mockClock) nowInternal() time.Time {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.now
}

// Now returns the current wall time on the mock clock.
func (m *mockClock) Now() time.Time {
	m.callHook(HookBeforeNow)
	t := m.nowInternal()
	m.callHook(HookAfterNow)
	return t
}

// Since returns time since the mock clocks wall time.
func (m *mockClock) Since(t time.Time) time.Duration {
	return m.nowInternal().Sub(t)
}

// Sleep pauses the goroutine for the given duration on the mock clock.
// The clock must be moved forward in a separate goroutine.
func (m *mockClock) Sleep(d time.Duration) {
	<-m.After(d)
}

// Tick is a convenience function for Ticker().
// It will return a ticker channel that cannot be stopped.
func (m *mockClock) Tick(d time.Duration) <-chan time.Time {
	return m.Ticker(d).C
}

// Ticker creates a new instance of Ticker.
func (m *mockClock) Ticker(d time.Duration) *Ticker {
	m.mu.Lock()
	defer m.mu.Unlock()
	// Use a buffered channel so we don't block in tests
	ch := make(chan time.Time, 1000)
	t := &Ticker{
		C:    ch,
		c:    ch,
		mock: m,
		d:    d,
		next: m.now.Add(d),
	}
	m.timers = append(m.timers, (*internalTicker)(t))
	return t
}

func (m *mockClock) timerInternal(d time.Duration) *Timer {
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
	m.callHookNoLock(HookBeforeTimer)

	if !t.next.After(m.now) {
		t.c <- m.now
	} else {
		m.timers = append(m.timers, (*internalTimer)(t))
	}

	return t
}

// Timer creates a new instance of Timer.
func (m *mockClock) Timer(d time.Duration) *Timer {
	tmr := m.timerInternal(d)
	m.callHook(HookAfterTimer)
	return tmr
}

func (m *mockClock) removeClockTimer(t clockTimer) {
	m.mu.Lock()
	defer m.mu.Unlock()
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

func hookPlaceMatches(c *hookCall) bool {
	if c.fn == nil && c.file == nil {
		return true
	}

	pcs := make([]uintptr, 100)
	n := runtime.Callers(1, pcs)
	stack := runtime.CallersFrames(pcs[:n])

	for {
		f, more := stack.Next()

		if skipThisPackageRegExp.MatchString(f.File) {
			if !more {
				break
			}
			continue
		}

		fnMatch := false
		fileMatch := false

		if c.fn != nil {
			if strings.HasSuffix(f.Function, "."+*c.fn) {
				fnMatch = true
			}
		}

		if c.file != nil {
			if strings.HasSuffix(f.File, *c.file) {
				fileMatch = true
			}
		}

		return (fnMatch || c.fn == nil) && (fileMatch || c.file == nil)
	}

	return false
}

func (m *mockClock) callHookNoLock(point HookPoint) {
	for _, c := range m.hooks[point] {
		if hookPlaceMatches(c) {
			c.h(m.now)
		}
	}
}

func (m *mockClock) callHook(point HookPoint) {
	m.mu.Lock()
	m.callHookNoLock(point)
	m.mu.Unlock()
}

// SetHook registers a hook to be called at a specific point
func (m *mockClock) SetHook(point HookPoint, h Hook) {
	m.SetHookExt(point, "", "", h)
}

// SetHookExt is an extended version of SetHook. It allows to set the hook for a
// specific function or a file or a function in a file. That means, if
// HookAfterTimer is set for a fn in file, it gets executed only if Timer is
// called directly from fn in file. An empty string is a wildcard.
//
// If there are multiple hooks to be executed, they are called in the
// order in which they were set
func (m *mockClock) SetHookExt(point HookPoint, fn string, file string, h Hook) {
	c := &hookCall{h: h}

	if fn != "" {
		c.fn = &fn
	}

	if file != "" {
		c.file = &file
	}

	m.mu.Lock()
	m.hooks[point] = append(m.hooks[point], c)
	m.mu.Unlock()
}

// DelHooks removes a hook from a fn and/ or file in the same way as SetHookExt
// sets them.
func (m *mockClock) DelHooks(point HookPoint, fn string, file string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hks := []*hookCall{}
	if fn == "" && file == "" {
		m.hooks[point] = hks
		return
	}

	for _, c := range m.hooks[point] {
		if (c.fn == nil && fn == "" || c.fn != nil && *c.fn == fn) &&
			(c.file == nil && file == "" || c.file != nil && *c.file == file) {
			continue
		}
		hks = append(hks, c)
	}

	m.hooks[point] = hks
}

// ResetHooks removes all hooks set
func (m *mockClock) ResetHooks() {
	m.mu.Lock()
	m.hooks = make(map[HookPoint][]*hookCall)
	m.mu.Unlock()
}

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
	sync.RWMutex
	C       <-chan time.Time
	c       chan time.Time
	timer   *time.Timer // realtime impl, if set
	next    time.Time   // next tick time
	mock    *mockClock  // mock clock, if set
	fn      func()      // AfterFunc function, if set
	stopped bool        // True if stopped, false if running
}

// Stop turns off the ticker.
func (t *Timer) Stop() bool {
	if t.timer != nil {
		return t.timer.Stop()
	}

	registered := !t.stopped
	t.mock.removeClockTimer((*internalTimer)(t))
	t.Lock()
	t.stopped = true
	t.Unlock()
	return registered
}

// Reset changes the expiry time of the timer
func (t *Timer) Reset(d time.Duration) bool {
	if t.timer != nil {
		return t.timer.Reset(d)
	}

	now := t.mock.nowInternal()
	next := now.Add(d)

	t.Lock()
	t.next = next
	registered := !t.stopped
	if t.stopped {
		t.mock.mu.Lock()
		t.mock.timers = append(t.mock.timers, (*internalTimer)(t))
		t.mock.mu.Unlock()
	}
	t.stopped = false
	t.Unlock()
	return registered
}

type internalTimer Timer

func (t *internalTimer) Next() time.Time {
	defer t.RUnlock()
	t.RLock()
	return t.next
}

func (t *internalTimer) Tick(now time.Time) {
	if t.fn != nil {
		t.fn()
	} else {
		t.c <- now
	}
	t.mock.removeClockTimer((*internalTimer)(t))
	t.Lock()
	t.stopped = true
	t.Unlock()
}

// Ticker holds a channel that receives "ticks" at regular intervals.
type Ticker struct {
	sync.RWMutex
	C      <-chan time.Time
	c      chan time.Time
	ticker *time.Ticker  // realtime impl, if set
	next   time.Time     // next tick time
	mock   *mockClock    // mock clock, if set
	d      time.Duration // time between ticks
}

// Stop turns off the ticker.
func (t *Ticker) Stop() {
	if t.ticker != nil {
		t.ticker.Stop()
	} else {
		t.mock.removeClockTimer((*internalTicker)(t))
	}
}

type internalTicker Ticker

func (t *internalTicker) Next() time.Time {
	defer t.RUnlock()
	t.RLock()
	return t.next
}

func (t *internalTicker) Tick(now time.Time) {
	t.c <- now
	t.Lock()
	t.next = now.Add(t.d)
	t.Unlock()
}
