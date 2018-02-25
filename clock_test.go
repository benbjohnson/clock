package clock

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sync/atomic"
	"testing"
	"time"
)

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
	var ok int32
	clock := NewMock()

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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var n int32
	clock := NewMock()
	ticker := clock.Ticker(10 * time.Second)

	// Create a channel to increment every 10 seconds.
	go func() {
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()

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
	ticker.Stop()
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

	// Move clock forward.
	for i := 0; i < 10; i++ {
		clock.Add(time.Microsecond)
	}
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
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var n int32
	clock := NewMock()

	// Create a channel to increment every second.
	ticker := clock.Ticker(1 * time.Second)
	go func() {
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
			select {
			case <-ctx.Done():
				return
			default:
			}
		}
	}()
	runtime.Gosched()

	// Move clock forward.
	clock.Add(5 * time.Second)
	if atomic.LoadInt32(&n) != 5 {
		t.Fatalf("expected 5, got: %d", atomic.LoadInt32(&n))
	}

	ticker.Stop()

	// Move clock forward again.
	clock.Add(5 * time.Second)
	if atomic.LoadInt32(&n) != 5 {
		t.Fatalf("still expected 5, got: %d", atomic.LoadInt32(&n))
	}
}

// Ensure that multiple tickers can be used together.
func TestMock_Ticker_Multi(t *testing.T) {
	clock := NewMock()

	ready := make(chan struct{})
	expect := make(chan struct{}, 500)
	received := 0
	go func() {
		a := clock.Ticker(1 * time.Microsecond)
		b := clock.Ticker(3 * time.Microsecond)
		ready <- struct{}{}
		for {
			select {
			case <-a.C:
				expect <- struct{}{}
			case <-b.C:
				for i := 0; i < 100; i++ {
					expect <- struct{}{}
				}
			}
		}
	}()
	<-ready

	waitfor := func(r int) {
		for received < r {
			<-expect
			received++
		}
	}

	// Move clock forward 1ms at a time, to avoid overflowing any
	// of the ticker channels.
	for i := 0; i < 10; i++ {
		clock.Add(1 * time.Microsecond)
	}
	waitfor(310)
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

func ExampleMock_Ticker() {
	// Create a new mock clock.
	clock := NewMock()
	var count int32

	ready := make(chan struct{})
	// Increment count every mock second.
	go func() {
		ticker := clock.Ticker(1 * time.Second)
		close(ready)
		for {
			<-ticker.C
			atomic.AddInt32(&count, 1)
		}
	}()
	<-ready

	// Move the clock forward 10 seconds and print the new value.
	clock.Add(10 * time.Second)
	fmt.Printf("Count is %d after 10 seconds\n", atomic.LoadInt32(&count))

	// Move the clock forward 5 more seconds and print the new value.
	clock.Add(5 * time.Second)
	fmt.Printf("Count is %d after 15 seconds\n", atomic.LoadInt32(&count))

	// Output:
	// Count is 10 after 10 seconds
	// Count is 15 after 15 seconds
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

func warn(v ...interface{})              { fmt.Fprintln(os.Stderr, v...) }
func warnf(msg string, v ...interface{}) { fmt.Fprintf(os.Stderr, msg+"\n", v...) }
