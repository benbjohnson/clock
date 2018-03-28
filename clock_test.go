package clock

import (
	"context"
	"fmt"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

func blockUntil(r int, ch <-chan struct{}) {
	for r > 0 {
		<-ch
		r--
	}
}

// Ensure that the mock's After channel sends at the correct time.
func TestMock_After(t *testing.T) {
	var ok int32
	clock := NewMock()

	// Create a channel to execute after 10 mock seconds.
	ch0 := clock.After(10 * time.Second)
	ch1 := make(chan struct{})
	go func() {
		<-ch0
		atomic.StoreInt32(&ok, 1)
		ch1 <- struct{}{}
	}()

	// Move clock forward to just before the time.
	clock.Add(9 * time.Second)
	select {
	case <-ch1:
		t.Fatal("too early")
	default:
	}
	if atomic.LoadInt32(&ok) == 1 {
		t.Fatal("too early")
	}

	// Move clock forward to the after channel's time.
	clock.Add(1 * time.Second)
	<-ch1
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
	runtime.Gosched()

	// Stop timer & move clock forward.
	if !timer.Stop() {
		t.Fatal("stop failed")
	}
	clock.Add(10 * time.Second)
	runtime.Gosched()
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

func TestMock_Since(t *testing.T) {
	clock := NewMock()

	beginning := clock.Now()
	clock.Add(500 * time.Second)
	if since := clock.Since(beginning); since.Seconds() != 500 {
		t.Fatalf("expected 500 since beginning, actually: %v", since.Seconds())
	}
}

// Ensure that the mock can sleep for the correct time.
func TestMock_Sleep(t *testing.T) {
	// This test repeats until a perfect execution happens.
	for !testMock_Sleep(t) {
		time.Sleep(time.Millisecond)
	}
}

func testMock_Sleep(t *testing.T) bool {
	ok := make(chan struct{})
	clock := NewMock()
	start := clock.Now()

	// Create a channel to execute after 10 mock seconds.
	go func() {
		clock.Sleep(10 * time.Second)
		ok <- struct{}{}
	}()
	// This test is inherently racey because we need the Sleep()
	// call to take the current time before calling Set below, and
	// we have no way to do that.
	runtime.Gosched()

	// Move clock forward to just before the sleep duration.
	clock.Set(start.Add(9 * time.Second))
	select {
	case <-ok:
		t.Fatal("too early")
	default:
	}

	// Move clock forward to the after the sleep duration.
	clock.Set(start.Add(10 * time.Second))
	select {
	case <-ok:
		return true
	default:
		return false
	}
}

// Ensure that the mock's Tick channel sends at the correct time.
func TestMock_Tick(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clock := NewMock()
	ticker := clock.Ticker(10 * time.Second)
	recv := make(chan struct{}, 4)

	// Create a channel to increment every 10 seconds.
	go func() {
		for {
			select {
			case <-ticker.C:
				recv <- struct{}{}
			case <-ctx.Done():
				return
			}
		}
	}()

	// Move clock forward to just before the first tick.
	clock.Add(9 * time.Second)
	if r := len(recv); r != 0 {
		t.Fatalf("expected 0, got %d", r)
	}

	// Move clock forward to the start of the first tick.
	clock.Add(1 * time.Second)
	<-recv

	// Move clock forward over several ticks.
	clock.Add(30 * time.Second)
	<-recv
	<-recv
	<-recv
	ticker.Stop()
}

// Ensure that the mock's Ticker channel sends at the correct time.
func TestMock_Ticker(t *testing.T) {
	// This test repeats until a perfect execution happens.
	for !testMock_Ticker() {
	}
}

func testMock_Ticker() bool {
	const size = 10
	received := 0
	recv := make(chan struct{}, size)
	clock := NewMock()
	ticker, drops := clock.TestTicker(1 * time.Microsecond)

	// Create a channel to increment every microsecond.
	go func() {
		for {
			select {
			case <-ticker.C:
				received++
				recv <- struct{}{}
			case <-drops:
				recv <- struct{}{}
			}
		}
	}()

	// Move clock forward.
	for i := 0; i < size; i++ {
		clock.Add(time.Microsecond)
	}
	blockUntil(size, recv)
	ticker.Stop()
	return received == size
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
	recv := make(chan struct{}, 50)
	clock := NewMock()

	// Create a channel to increment every second.
	ticker, drops := clock.TestTicker(1 * time.Second)
	go func() {
		for {

			select {
			case <-ticker.C:
				recv <- struct{}{}
			case _, ok := <-drops:
				if !ok {
					return
				}
				recv <- struct{}{}
			}
		}
	}()

	// Move clock forward.
	clock.Add(5 * time.Second)
	blockUntil(5, recv)

	ticker.Stop()

	// Move clock forward again.
	clock.Add(5 * time.Second)
	select {
	case <-recv:
		t.Fatalf("unexpected notification")
	default:
	}
}

// Ensure that the mock's Stop method wakes sleepers.
func TestMock_Mock_Stop(t *testing.T) {
	mock := NewMock()
	clock := Clock(mock)

	ch := make(chan struct{})
	go func() {
		clock.Sleep(time.Second)
		ch <- struct{}{}
	}()
	mock.Stop()
	<-ch

	// Timers finish immediately
	timer := clock.Timer(time.Hour)
	<-timer.C

	// Tickers finish immediately
	ticker := clock.Ticker(time.Hour)
	<-ticker.C

	// Sleep returns immediately
	clock.Sleep(time.Hour)
}

// Ensure that multiple tickers can be used together.
func TestMock_Ticker_Multi(t *testing.T) {
	// This test repeats until a perfect execution happens.
	for !testMock_Ticker_Multi() {
	}
}

// This test is repeated until it succeeds with no dropped
// notifications, proving that the multi-ticker logic is correct.
func testMock_Ticker_Multi() bool {
	const (
		duration = 10
		unit     = time.Microsecond
		aCount   = 1
		bCount   = 100
		aPeriod  = 1
		bPeriod  = 3
		aTicks   = duration / aPeriod
		bTicks   = duration / bPeriod
		expect   = bTicks*bCount + aTicks*aCount
	)
	mock := NewMock()
	a, aDrop := mock.TestTicker(aPeriod * unit)
	b, bDrop := mock.TestTicker(bPeriod * unit)
	recv := make(chan struct{}, expect)
	aReceived := 0
	bReceived := 0
	sendN := func(n int) {
		for n > 0 {
			recv <- struct{}{}
			n--
			runtime.Gosched()
		}
	}

	go func() {
		for {
			select {
			case <-a.C:
				aReceived++
				go sendN(aCount)
			case _, ok := <-aDrop:
				if !ok {
					return
				}
				go sendN(aCount)
			case <-b.C:
				bReceived++
				go sendN(bCount)
			case _, ok := <-bDrop:
				if !ok {
					return
				}
				go sendN(bCount)
			}
			runtime.Gosched()
		}
	}()

	// Move clock forward 1ms at a time, to avoid overflowing any
	// of the ticker channels.
	for i := 0; i < duration; i++ {
		mock.Add(unit)
	}

	blockUntil(expect, recv)
	a.Stop()
	b.Stop()

	return aReceived == aTicks && bReceived == bTicks
}

func ExampleMock_After() {
	// Create a new mock clock.
	clock := NewMock()
	var count int32

	ready := make(chan struct{})
	// Create a channel to execute after 10 mock seconds.
	go func() {
		ch := clock.After(10 * time.Second)
		close(ready)
		<-ch
		atomic.StoreInt32(&count, 100)
	}()
	<-ready

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), atomic.LoadInt32(&count))

	// Move the clock forward 5 seconds and print the value again.
	clock.Add(5 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), atomic.LoadInt32(&count))

	// Move the clock forward 5 seconds to the tick time and check the value.
	clock.Add(5 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), atomic.LoadInt32(&count))

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:05 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_AfterFunc() {
	// Create a new mock clock.
	clock := NewMock()
	count := 0

	// Execute a function after 10 mock seconds.
	clock.AfterFunc(10*time.Second, func() {
		count = 100
	})
	runtime.Gosched()

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Move the clock forward 10 seconds and print the new value.
	clock.Add(10 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_Sleep() {
	// Create a new mock clock.
	clock := NewMock()
	var count int32

	// Execute a function after 10 mock seconds.
	go func() {
		clock.Sleep(10 * time.Second)
		atomic.StoreInt32(&count, 100)
	}()
	runtime.Gosched()

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), atomic.LoadInt32(&count))

	// Move the clock forward 10 seconds and print the new value.
	clock.Add(10 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), atomic.LoadInt32(&count))

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_Timer() {
	// Create a new mock clock.
	clock := NewMock()
	var count int32

	ready := make(chan struct{})
	// Increment count after a mock second.
	go func() {
		timer := clock.Timer(1 * time.Second)
		close(ready)
		<-timer.C
		atomic.AddInt32(&count, 1)
	}()
	<-ready

	// Move the clock forward 10 seconds and print the new value.
	clock.Add(10 * time.Second)
	fmt.Printf("Count is %d after 10 seconds\n", atomic.LoadInt32(&count))

	// Output:
	// Count is 1 after 10 seconds
}
