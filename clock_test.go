package clock_test

import (
	"fmt"
	"os"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/benbjohnson/clock"
)

// Ensure that the clock's After channel sends at the correct time.
func TestClock_After(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		t.Fatal("too late")
	}()
	runtime.Gosched()

	<-clock.New().After(20 * time.Millisecond)
	runtime.Gosched()
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure that the clock's AfterFunc executes at the correct time.
func TestClock_AfterFunc(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		t.Fatal("too late")
	}()
	runtime.Gosched()

	var wg sync.WaitGroup
	wg.Add(1)
	runtime.Gosched()
	clock.New().AfterFunc(20*time.Millisecond, func() {
		wg.Done()
	})
	runtime.Gosched()
	wg.Wait()
	runtime.Gosched()
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure that the clock's time matches the standary library.
func TestClock_Now(t *testing.T) {
	a := time.Now().Round(time.Second)
	b := clock.New().Now().Round(time.Second)
	if !a.Equal(b) {
		t.Errorf("not equal: %s != %s", a, b)
	}
}

// Ensure that the clock sleeps for the appropriate amount of time.
func TestClock_Sleep(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		t.Fatal("too late")
	}()

	clock.New().Sleep(20 * time.Millisecond)
	runtime.Gosched()
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure that the clock ticks correctly.
func TestClock_Tick(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		t.Fatal("too late")
	}()

	c := clock.New().Tick(20 * time.Millisecond)
	runtime.Gosched()
	<-c
	runtime.Gosched()
	<-c
	runtime.Gosched()
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure that the clock's ticker ticks correctly.
func TestClock_Ticker(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(50 * time.Millisecond)
		t.Fatal("too late")
	}()

	ticker := clock.New().Ticker(20 * time.Millisecond)
	runtime.Gosched()
	<-ticker.C
	runtime.Gosched()
	<-ticker.C
	runtime.Gosched()
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure that the clock's ticker can stop correctly.
func TestClock_Ticker_Stp(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()
	runtime.Gosched()

	ticker := clock.New().Ticker(20 * time.Millisecond)
	<-ticker.C
	runtime.Gosched()
	ticker.Stop()
	runtime.Gosched()
	select {
	case <-ticker.C:
		t.Fatal("unexpected send")
	case <-time.After(30 * time.Millisecond):
	}
}

// Ensure that the clock's timer waits correctly.
func TestClock_Timer(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		t.Fatal("too late")
	}()

	timer := clock.New().Timer(20 * time.Millisecond)
	runtime.Gosched()
	<-timer.C
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure that the clock's timer can be stopped.
func TestClock_Timer_Stop(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()

	timer := clock.New().Timer(20 * time.Millisecond)
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
	clock := clock.NewMock()

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

// Ensure that the mock's AfterFunc executes at the correct time.
func TestMock_AfterFunc(t *testing.T) {
	var ok int32
	clock := clock.NewMock()

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
	clock := clock.NewMock()
	timer := clock.AfterFunc(10*time.Second, func() {
		t.Fatal("unexpected function execution")
	})

	// Stop timer & move clock forward.
	timer.Stop()
	clock.Add(10 * time.Second)
	runtime.Gosched()
}

// Ensure that the mock's current time can be changed.
func TestMock_Now(t *testing.T) {
	clock := clock.NewMock()
	if now := clock.Now(); !now.Equal(time.Unix(0, 0).UTC()) {
		t.Fatalf("expected epoch, got: ", now)
	}

	// Add 10 seconds and check the time.
	clock.Add(10 * time.Second)
	if now := clock.Now(); !now.Equal(time.Unix(10, 0).UTC()) {
		t.Fatalf("expected epoch, got: ", now)
	}
}

// Ensure that the mock can sleep for the correct time.
func TestMock_Sleep(t *testing.T) {
	var ok int32
	clock := clock.NewMock()

	// Create a channel to execute after 10 mock seconds.
	go func() {
		clock.Sleep(10 * time.Second)
		atomic.StoreInt32(&ok, 1)
	}()
	runtime.Gosched()

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
	clock := clock.NewMock()

	// Create a channel to increment every 10 seconds.
	go func() {
		tick := clock.Tick(10 * time.Second)
		for {
			<-tick
			atomic.AddInt32(&n, 1)
		}
	}()
	runtime.Gosched()

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
	clock := clock.NewMock()

	// Create a channel to increment every microsecond.
	go func() {
		ticker := clock.Ticker(1 * time.Microsecond)
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
		}
	}()
	runtime.Gosched()

	// Move clock forward.
	clock.Add(10 * time.Microsecond)
	if atomic.LoadInt32(&n) != 10 {
		t.Fatalf("unexpected: %d", n)
	}
}

// Ensure that the mock's Ticker channel won't block if not read from.
func TestMock_Ticker_Overflow(t *testing.T) {
	clock := clock.NewMock()
	ticker := clock.Ticker(1 * time.Microsecond)
	clock.Add(10 * time.Microsecond)
	ticker.Stop()
}

// Ensure that the mock's Ticker can be stopped.
func TestMock_Ticker_Stop(t *testing.T) {
	var n int32
	clock := clock.NewMock()

	// Create a channel to increment every second.
	ticker := clock.Ticker(1 * time.Second)
	go func() {
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
		}
	}()
	runtime.Gosched()

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
	clock := clock.NewMock()

	//
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
	runtime.Gosched()

	// Move clock forward.
	clock.Add(10 * time.Microsecond)
	if atomic.LoadInt32(&n) != 310 {
		t.Fatalf("unexpected: %d", n)
	}
}

func warn(v ...interface{})              { fmt.Fprintln(os.Stderr, v...) }
func warnf(msg string, v ...interface{}) { fmt.Fprintf(os.Stderr, msg+"\n", v...) }
