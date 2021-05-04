package clock

import (
	"fmt"
	"sync/atomic"
	"testing"
	"time"
)

// Ensure that the mock's After channel sends at the correct time.
func TestMock_After(t *testing.T) {
	var ok int32
	clock := NewUnsynchronizedMock()

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
	mock := NewUnsynchronizedMock()
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
	clock := NewUnsynchronizedMock()

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
	clock := NewUnsynchronizedMock()
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
	clock := NewUnsynchronizedMock()
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
	clock := NewUnsynchronizedMock()

	beginning := clock.Now()
	clock.Add(500 * time.Second)
	if since := clock.Since(beginning); since.Seconds() != 500 {
		t.Fatalf("expected 500 since beginning, actually: %v", since.Seconds())
	}
}

// Ensure that the mock can sleep for the correct time.
func TestMock_Sleep(t *testing.T) {
	var ok int32
	clock := NewMock(t, 1)

	// Create a channel to execute after 10 mock seconds.
	go func() {
		clock.Sleep(10 * time.Second)
		atomic.StoreInt32(&ok, 1)
		clock.Confirm()
	}()

	// Move clock forward to just before the sleep duration.
	clock.Add(9 * time.Second)
	if atomic.LoadInt32(&ok) == 1 {
		t.Fatal("too early")
	}

	// Move clock forward to after the sleep duration.
	clock.Add(1*time.Second, ExpectUpcomingConfirms(1))
	if atomic.LoadInt32(&ok) == 0 {
		t.Fatal("too late")
	}
}

// Ensure that the mock's Tick channel sends at the correct time.
func TestMock_Tick(t *testing.T) {
	var n int32
	clock := NewMock(t, 1)

	// Create a channel to increment every 10 seconds.
	go func() {
		tick := clock.Tick(10 * time.Second)
		for {
			<-tick
			atomic.AddInt32(&n, 1)
			clock.Confirm()
		}
	}()

	// Move clock forward to just before the first tick.
	clock.Add(9 * time.Second)
	if atomic.LoadInt32(&n) != 0 {
		t.Fatalf("expected 0, got %d", n)
	}

	// Move clock forward to the start of the first tick.
	clock.Add(1*time.Second, ExpectUpcomingConfirms(1))
	if atomic.LoadInt32(&n) != 1 {
		t.Fatalf("expected 1, got %d", n)
	}

	// Move clock forward over several ticks.
	clock.Add(30*time.Second, ExpectUpcomingConfirms(3))
	if atomic.LoadInt32(&n) != 4 {
		t.Fatalf("expected 4, got %d", n)
	}
}

// Ensure that the mock's Ticker channel sends at the correct time.
func TestMock_Ticker(t *testing.T) {
	var n int32
	clock := NewMock(t, 1)

	// Create a channel to increment every microsecond.
	go func() {
		ticker := clock.NewTicker(1 * time.Microsecond)
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
			clock.Confirm()
		}
	}()

	// Move clock forward.
	clock.Add(10*time.Microsecond, ExpectUpcomingConfirms(10))
	if atomic.LoadInt32(&n) != 10 {
		t.Fatalf("unexpected: %d", n)
	}
}

// Ensure that the mock's Ticker channel won't block if not read from.
func TestMock_Ticker_Overflow(t *testing.T) {
	clock := NewUnsynchronizedMock()
	ticker := clock.NewTicker(1 * time.Microsecond)
	clock.Add(10 * time.Microsecond)
	ticker.Stop()
}

// Ensure that the mock's Ticker can be stopped.
func TestMock_Ticker_Stop(t *testing.T) {
	var n int32
	clock := NewMock(t, 1)

	// Create a channel to increment every second.
	ticker := clock.NewTicker(1 * time.Second)
	go func() {
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
			clock.Confirm()
		}
	}()

	// Move clock forward.
	clock.Add(5*time.Second, ExpectUpcomingConfirms(5))
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

func TestMock_Ticker_Reset(t *testing.T) {
	var n int32
	clock := NewMock(t, 1)

	ticker := clock.NewTicker(5 * time.Second)
	defer ticker.Stop()

	go func() {
		for {
			<-ticker.C
			atomic.AddInt32(&n, 1)
			clock.Confirm()
		}
	}()

	// Move clock forward.
	clock.Add(10*time.Second, ExpectUpcomingConfirms(2))
	if atomic.LoadInt32(&n) != 2 {
		t.Fatalf("expected 2, got: %d", n)
	}

	clock.Add(4 * time.Second)
	ticker.Reset(5 * time.Second)

	// Advance the remaining second
	clock.Add(1 * time.Second)

	if atomic.LoadInt32(&n) != 2 {
		t.Fatalf("expected 2, got: %d", n)
	}

	// Advance the remaining 4 seconds from the previous tick
	clock.Add(4*time.Second, ExpectUpcomingConfirms(1))

	if atomic.LoadInt32(&n) != 3 {
		t.Fatalf("expected 3, got: %d", n)
	}
}

// Ensure that multiple tickers can be used together.
func TestMock_Ticker_Multi(t *testing.T) {
	var n int32
	clock := NewMock(t, 2)

	go func() {
		a := clock.NewTicker(1 * time.Microsecond)
		b := clock.NewTicker(3 * time.Microsecond)

		for {
			select {
			case <-a.C:
				atomic.AddInt32(&n, 1)
			case <-b.C:
				atomic.AddInt32(&n, 100)
			}
		}
	}()

	// Move clock forward.
	clock.Add(10*time.Microsecond, ExpectUpcomingConfirms(13))
	if atomic.LoadInt32(&n) != 310 {
		t.Fatalf("unexpected: %d", n)
	}
}

func TestMock_Interface(t *testing.T) {
	var c Clock = NewUnsynchronizedMock()
	SetSystemClock(c)
}

func ExampleMock_After() {
	// Create a new mock clock.
	mock := NewMock(nil, 1)
	SetSystemClock(mock)
	count := 0

	// Create a channel to execute after 10 mock seconds.
	go func() {
		ch := After(10 * time.Second)
		<-ch
		count = 100
		Confirm()
	}()
	mock.WaitForStart()

	// Print the starting value.
	fmt.Printf("%s: %d\n", Now().UTC(), count)

	// Move the clock forward 5 seconds and print the value again.
	mock.Add(5 * time.Second)
	fmt.Printf("%s: %d\n", Now().UTC(), count)

	// Move the clock forward 5 seconds to the tick time and check the value.
	mock.Add(5*time.Second, ExpectUpcomingConfirms(1))
	fmt.Printf("%s: %d\n", Now().UTC(), count)

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:05 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_AfterFunc() {
	// Create a new mock clock.
	mock := NewMock(nil, 1)
	SetSystemClock(mock)
	count := 0

	// Execute a function after 10 mock seconds.
	AfterFunc(10*time.Second, func() {
		count = 100
		Confirm()
	})

	// Print the starting value.
	fmt.Printf("%s: %d\n", Now().UTC(), count)

	// Move the clock forward 10 seconds and print the new value.
	mock.Add(10*time.Second, ExpectUpcomingConfirms(1))

	fmt.Printf("%s: %d\n", Now().UTC(), count)

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_Sleep() {
	// Create a new mock clock.
	mock := NewMock(nil, 1)
	SetSystemClock(mock)
	count := 0

	// Execute a function after 10 mock seconds.
	go func() {
		Sleep(10 * time.Second)
		count = 100
		Confirm()
	}()
	mock.WaitForStart()

	// Print the starting value.
	fmt.Printf("%s: %d\n", Now().UTC(), count)

	// Move the clock forward 10 seconds and print the new value.
	mock.Add(10*time.Second, ExpectUpcomingConfirms(1))
	fmt.Printf("%s: %d\n", Now().UTC(), count)

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleMock_Ticker() {
	// Create a new mock clock.
	mock := NewMock(nil, 1)
	SetSystemClock(mock)
	count := 0

	// Increment count every mock second.
	go func() {
		ticker := NewTicker(1 * time.Second)
		for {
			<-ticker.C
			count++
			Confirm()
		}
	}()

	// Move the clock forward 10 seconds and print the new value.
	mock.Add(10*time.Second, ExpectUpcomingConfirms(10))
	fmt.Printf("Count is %d after 10 seconds\n", count)

	// Move the clock forward 5 more seconds and print the new value.
	mock.Add(5*time.Second, ExpectUpcomingConfirms(5))
	fmt.Printf("Count is %d after 15 seconds\n", count)

	// Output:
	// Count is 10 after 10 seconds
	// Count is 15 after 15 seconds
}

func ExampleMock_Timer() {
	// Create a new mock clock.
	mock := NewMock(nil, 1)
	SetSystemClock(mock)
	count := 0

	// Increment count after a mock second.
	go func() {
		timer := NewTimer(1 * time.Second)
		<-timer.C
		count++
		Confirm()
	}()

	// Move the clock forward 10 seconds and print the new value.
	mock.Add(10*time.Second, ExpectUpcomingConfirms(10))
	fmt.Printf("Count is %d after 10 seconds\n", count)

	// Output:
	// Count is 1 after 10 seconds
}
