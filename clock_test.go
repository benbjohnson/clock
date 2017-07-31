package clock

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

type aBool struct {
	uint32
}

func (o *aBool) set() {
	atomic.StoreUint32(&o.uint32, 1)
}
func (o *aBool) isSet() bool {
	return atomic.LoadUint32(&o.uint32) == 1
}

// Ensure that the clock's After channel sends at the correct time.
func TestClock_After(t *testing.T) {
	tooLate := make(chan struct{})
	ok := new(aBool)
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok.set()
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		close(tooLate)
	}()
	gosched()

	select {
	case <-New().After(20 * time.Millisecond):
		if !ok.isSet() {
			t.Fatal("too early")
		}
	case <-tooLate:
		t.Fatal("too late")
	}
}

// Ensure that the clock's AfterFunc executes at the correct time.
func TestClock_AfterFunc(t *testing.T) {
	finished := make(chan struct{})
	tooLate := make(chan struct{})
	ok := new(aBool)
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok.set()
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		close(tooLate)
	}()
	gosched()

	New().AfterFunc(20*time.Millisecond, func() {
		close(finished)
	})
	select {
	case <-finished:
		if !ok.isSet() {
			t.Fatal("too early")
		}
	case <-tooLate:
		t.Fatal("too late")
	}
}

// Ensure that the clock's time matches the standary library.
func TestClock_Now(t *testing.T) {
	a := time.Now().Round(time.Second)
	b := New().Now().Round(time.Second)
	if !a.Equal(b) {
		t.Errorf("not equal: %s != %s", a, b)
	}
}

// Ensure that the clock sleeps for the appropriate amount of time.
func TestClock_Sleep(t *testing.T) {
	ok := new(aBool)
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok.set()
	}()
	gosched()

	New().Sleep(20 * time.Millisecond)
	select {
	case <-time.After(30 * time.Millisecond):
		t.Fatal("too late")
	default:
		if !ok.isSet() {
			t.Fatal("too early")
		}
	}
}

// Ensure that the clock's ticker ticks correctly.
func TestClock_Ticker(t *testing.T) {
	ticker := New().Ticker(100 * time.Millisecond)
	start := time.Now()

	for {
		select {
		case <-ticker.C:
			if time.Since(start) <= 100*time.Millisecond {
				t.Fatal("too early")
			}
			return
		case <-time.After(103 * time.Millisecond):
			t.Fatal("too late")
		}
	}
}

// Ensure that the clock's ticker can stop correctly.
func TestClock_Ticker_Stp(t *testing.T) {
	ticker := New().Ticker(20 * time.Millisecond)
	<-ticker.C
	ticker.Stop()

	select {
	case <-ticker.C:
		t.Fatal("unexpected tick after stop")
	case <-time.After(30 * time.Millisecond):
	}
}

// Ensure that the clock's timer waits correctly.
func TestClock_Timer(t *testing.T) {
	ok := new(aBool)
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok.set()
	}()
	gosched()

	timer := New().Timer(20 * time.Millisecond)
	select {
	case <-timer.C:
		if !ok.isSet() {
			t.Fatal("too early")
		}
	case <-time.After(30 * time.Millisecond):
		t.Fatal("too late")
	}
}

// Ensure that the clock's timer can be stopped.
func TestClock_Timer_Stop(t *testing.T) {
	timer := New().Timer(20 * time.Millisecond)
	timer.Stop()
	select {
	case <-timer.C:
		t.Fatal("unexpected send")
	case <-time.After(30 * time.Millisecond):
	}
}

// Ensure that the mock's After channel sends at the correct time.
func TestMock_After(t *testing.T) {
	var ok int32
	clock := NewMock()

	// Create a channel to execute after 10 mock seconds.
	ch := clock.After(10 * time.Second)
	go func(ch <-chan time.Time) {
		<-ch
		atomic.StoreInt32(&ok, 1)
	}(ch)

	// Move clock forward to just before the time.
	clock.Add(9 * time.Second)
	if atomic.LoadInt32(&ok) == 1 {
		t.Fatal("too early")
	}

	// Move clock forward to the after channel's time.
	clock.Add(1 * time.Second)
	if atomic.LoadInt32(&ok) == 0 {
		t.Fatal("too late")
	}
}

// Ensure that the mock's After channel doesn't block on write.
func TestMock_UnusedAfter(t *testing.T) {
	mock := NewMock()
	mock.After(1 * time.Millisecond)

	done := make(chan bool, 1)
	go func() {
		mock.Add(1 * time.Second)
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(1 * time.Second):
		t.Fatal("mock.Add hung")
	}
}

// Ensure that the mock's AfterFunc executes at the correct time.
func TestMock_AfterFunc(t *testing.T) {
	var ok int32
	clock := NewMock()

	// Execute function after duration.
	clock.AfterFunc(10*time.Second, func() {
		atomic.StoreInt32(&ok, 1)
	})

	// Move clock forward to just before the time.
	clock.Add(9 * time.Second)
	if atomic.LoadInt32(&ok) == 1 {
		t.Fatal("too early")
	}

	// Move clock forward to the after channel's time.
	clock.Add(1 * time.Second)
	if atomic.LoadInt32(&ok) == 0 {
		t.Fatal("too late")
	}
}

// Ensure that the mock's AfterFunc doesn't execute if stopped.
func TestMock_AfterFunc_Stop(t *testing.T) {
	// Execute function after duration.
	clock := NewMock()
	timer := clock.AfterFunc(10*time.Second, func() {
		t.Fatal("unexpected function execution")
	})
	gosched()

	// Stop timer & move clock forward.
	timer.Stop()
	clock.Add(10 * time.Second)
	gosched()
}

// Ensure that the mock's current time can be changed.
func TestMock_Now(t *testing.T) {
	clock := NewMock()
	if now := clock.Now(); !now.Equal(time.Unix(0, 0)) {
		t.Fatalf("expected epoch, got: %v", now)
	}

	// Add 10 seconds and check the time.
	clock.Add(10 * time.Second)
	if now := clock.Now(); !now.Equal(time.Unix(10, 0)) {
		t.Fatalf("expected epoch, got: %v", now)
	}
}

// Ensure that the mock can sleep for the correct time.
func TestMock_Sleep(t *testing.T) {
	var ok int32
	clock := NewMock()

	// Create a channel to execute after 10 mock seconds.
	go func() {
		clock.Sleep(10 * time.Second)
		atomic.StoreInt32(&ok, 1)
	}()
	gosched()

	// Move clock forward to just before the sleep duration.
	clock.Add(9 * time.Second)
	if atomic.LoadInt32(&ok) == 1 {
		t.Fatal("too early")
	}

	// Move clock forward to the after the sleep duration.
	clock.Add(1 * time.Second)
	if atomic.LoadInt32(&ok) == 0 {
		t.Fatal("too late")
	}
}

// Ensure that the mock's Tick channel sends at the correct time.
func TestMock_Tick(t *testing.T) {
	var n int32
	clock := NewMock()

	// Create a channel to increment every 10 seconds.
	go func() {
		tick := clock.Tick(10 * time.Second)
		for {
			<-tick
			atomic.AddInt32(&n, 1)
		}
	}()
	gosched()

	// Move clock forward to just before the first tick.
	clock.Add(9 * time.Second)
	if atomic.LoadInt32(&n) != 0 {
		t.Fatalf("expected 0, got %d", n)
	}

	// Move clock forward to the start of the first tick.
	clock.Add(1 * time.Second)
	if atomic.LoadInt32(&n) != 1 {
		t.Fatalf("expected 1, got %d", n)
	}

	// Move clock forward over several ticks.
	clock.Add(30 * time.Second)
	if atomic.LoadInt32(&n) != 4 {
		t.Fatalf("expected 4, got %d", n)
	}
}

// Ensure that the mock's Ticker channel sends at the correct time.
func TestMock_Ticker(t *testing.T) {
	var n int32
	clock := NewMock()

	// Create a channel to increment every microsecond.
	go func() {
		ticker := clock.Ticker(1 * time.Microsecond)
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
		}
	}()
	gosched()

	// Move clock forward.
	clock.Add(10 * time.Microsecond)
	if atomic.LoadInt32(&n) != 10 {
		t.Fatalf("unexpected: %d", n)
	}
}

// Ensure that the mock's Ticker channel won't block if not read from.
func TestMock_Ticker_Overflow(t *testing.T) {
	clock := NewMock()
	ticker := clock.Ticker(1 * time.Microsecond)
	clock.Add(10 * time.Microsecond)
	ticker.Stop()
}

// Ensure that the mock's Ticker can be stopped.
func TestMock_Ticker_Stop(t *testing.T) {
	var n int32
	clock := NewMock()

	// Create a channel to increment every second.
	ticker := clock.Ticker(1 * time.Second)
	go func() {
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
		}
	}()
	gosched()

	// Move clock forward.
	clock.Add(5 * time.Second)
	if atomic.LoadInt32(&n) != 5 {
		t.Fatalf("expected 5, got: %d", n)
	}

	ticker.Stop()

	// Move clock forward again.
	clock.Add(5 * time.Second)
	if atomic.LoadInt32(&n) != 5 {
		t.Fatalf("still expected 5, got: %d", n)
	}
}

// Ensure that multiple tickers can be used together.
func TestMock_Ticker_Multi(t *testing.T) {
	var n int32
	clock := NewMock()

	go func() {
		a := clock.Ticker(1 * time.Microsecond)
		b := clock.Ticker(3 * time.Microsecond)

		for {
			select {
			case <-a.C:
				atomic.AddInt32(&n, 1)
			case <-b.C:
				atomic.AddInt32(&n, 100)
			}
		}
	}()
	gosched()

	// Move clock forward.
	clock.Add(10 * time.Microsecond)
	gosched()
	if atomic.LoadInt32(&n) != 310 {
		t.Fatalf("unexpected: %d", n)
	}
}

type count struct{ int32 }

func (c *count) set(d int32) { atomic.StoreInt32(&c.int32, d) }
func (c *count) load() int32 { return atomic.LoadInt32(&c.int32) }

func ExampleMock_After() {
	clock := NewMock()
	count := new(count)

	ready := make(chan struct{})
	// Create a channel to execute after 10 mock seconds.
	go func() {
		ch := clock.After(10 * time.Second)
		close(ready)
		<-ch
		count.set(100)
	}()
	<-ready

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count.load())

	// Move the clock forward 5 seconds and print the value again.
	clock.Add(5 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count.load())

	// Move the clock forward 5 seconds to the tick time and check the value.
	clock.Add(5 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count.load())

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:05 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_AfterFunc() {
	clock := NewMock()
	count := new(count)

	// Execute a function after 10 mock seconds.
	clock.AfterFunc(10*time.Second, func() {
		count.set(100)
	})
	gosched()

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count.load())

	// Move the clock forward 10 seconds and print the new value.
	clock.Add(10 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count.load())

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_Sleep() {
	clock := NewMock()
	count := new(count)

	// Execute a function after 10 mock seconds.
	go func() {
		clock.Sleep(10 * time.Second)
		count.set(100)
	}()
	gosched()

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count.load())

	// Move the clock forward 10 seconds and print the new value.
	clock.Add(10 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count.load())

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_Ticker() {
	clock := NewMock()
	count := new(count)

	ready := make(chan struct{})
	// Increment count every mock second.
	go func() {
		ticker := clock.Ticker(1 * time.Second)
		close(ready)
		for {
			<-ticker.C
			count.set(count.load() + 1)
		}
	}()
	<-ready

	// Move the clock forward 10 seconds and print the new value.
	clock.Add(10 * time.Second)
	fmt.Printf("Count is %d after 10 seconds\n", count.load())

	// Move the clock forward 5 more seconds and print the new value.
	clock.Add(5 * time.Second)
	fmt.Printf("Count is %d after 15 seconds\n", count.load())

	// Output:
	// Count is 10 after 10 seconds
	// Count is 15 after 15 seconds
}

func ExampleMock_Timer() {
	clock := NewMock()
	count := new(count)

	ready := make(chan struct{})
	// Increment count after a mock second.
	go func() {
		timer := clock.Timer(1 * time.Second)
		close(ready)
		<-timer.C
		count.set(count.load() + 1)

	}()
	<-ready

	// Move the clock forward 10 seconds and print the new value.
	clock.Add(10 * time.Second)
	fmt.Printf("Count is %d after 10 seconds\n", count.load())

	// Output:
	// Count is 1 after 10 seconds
}

func TestMock_Timer(t *testing.T) {
	clock := NewMock()

	tA := clock.Timer(0)
	clock.Add(time.Microsecond)
	<-tA.C
	wasRunning := tA.Reset(time.Millisecond)
	if wasRunning {
		t.Fatal("timer.Stop returned true after timer fired")
	}
	tX := clock.Timer(3 * time.Millisecond)
	clock.Add(2 * time.Millisecond)

	select {
	case <-tA.C:
	case <-tX.C:
		t.Fatal("3 < 1")
	}
}
