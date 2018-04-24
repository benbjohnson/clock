// Copyright (c) 2014 Ben Johnson

package clock

import (
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Ensure that the clock's After channel sends at the correct time.
func TestClock_After(t *testing.T) {
	clk := New()
	st := clk.Now()
	<-clk.After(100 * time.Millisecond)
	elapsed := clk.Now().Sub(st)
	if elapsed < 10*time.Millisecond {
		t.Fatalf("Expected more time to elapse for 100ms sleep than %s", elapsed)
	}
}

// Ensure that the clock's AfterFunc executes at the correct time.
func TestClock_AfterFunc(t *testing.T) {
	clk := New()
	st := clk.Now()
	var wg sync.WaitGroup
	wg.Add(1)
	clk.AfterFunc(100*time.Millisecond, func() {
		wg.Done()
	})
	wg.Wait()
	elapsed := clk.Now().Sub(st)
	if elapsed < 10*time.Millisecond {
		t.Fatalf("Expected more time to elapse for 100ms sleep than %s", elapsed)
	}
}

// Ensure that the clock's time matches the standary library.
func TestClock_Now(t *testing.T) {
	a := time.Now().Round(time.Minute)
	b := New().Now().Round(time.Minute)
	if !a.Equal(b) {
		t.Errorf("not equal: %s != %s", a, b)
	}
}

// Ensure that the clock sleeps for the appropriate amount of time.
func TestClock_Sleep(t *testing.T) {
	clk := New()
	st := clk.Now()
	clk.Sleep(100 * time.Millisecond)
	elapsed := clk.Now().Sub(st)
	if elapsed < 10*time.Millisecond {
		t.Fatalf("Expected more time to elapse for 100ms sleep than %s", elapsed)
	}
}

// Ensure that the clock ticks correctly.
func TestClock_Tick(t *testing.T) {
	clk := New()
	st := clk.Now()
	c := clk.Tick(50 * time.Millisecond)
	<-c
	<-c
	elapsed := clk.Now().Sub(st)
	if elapsed < 10*time.Millisecond {
		t.Fatalf("Expected more time to elapse for two 50ms ticks than %s", elapsed)
	}
}

// Ensure that the clock's ticker ticks correctly.
func TestClock_Ticker(t *testing.T) {
	clk := New()
	st := clk.Now()
	ticker := clk.Ticker(50 * time.Millisecond)
	<-ticker.C
	<-ticker.C
	elapsed := clk.Now().Sub(st)
	if elapsed < 10*time.Millisecond {
		t.Fatalf("Expected more time to elapse for two 50ms ticks than %s", elapsed)
	}
}

// Stop a ticker
func TestClock_Ticker_Stp(t *testing.T) {
	ticker := New().Ticker(20 * time.Millisecond)
	<-ticker.C
	ticker.Stop()
}

// Ensure that the clock's timer waits correctly.
func TestClock_Timer(t *testing.T) {
	clk := New()
	st := clk.Now()

	timer := clk.Timer(100 * time.Millisecond)
	<-timer.C

	elapsed := clk.Now().Sub(st)
	if elapsed < 10*time.Millisecond {
		t.Fatalf("Expected more time to elapse for a 100ms timer than %s", elapsed)
	}

	if timer.Stop() {
		t.Fatal("timer still running")
	}
}

// Ensure that the clock's timer can be stopped.
func TestClock_Timer_Stop(t *testing.T) {
	timer := New().Timer(20 * time.Millisecond)
	if !timer.Stop() {
		t.Fatal("timer not running")
	}
	if timer.Stop() {
		t.Fatal("timer wasn't cancelled")
	}
}

// Ensure that the clock's timer can be reset.
func TestClock_Timer_Reset(t *testing.T) {
	timer := New().Timer(10 * time.Millisecond)
	if !timer.Reset(20 * time.Millisecond) {
		t.Fatal("timer not running")
	}
	<-timer.C
}

// Ensure that the mock's After channel sends at the correct time.
func TestMock_After(t *testing.T) {
	var ok int32
	var wg sync.WaitGroup

	mock := NewMock()
	clock := mock.Clock()

	wg.Add(1)

	now := mock.Now()

	// Create a channel to execute after 10 mock seconds.
	ch := clock.After(10 * time.Second)
	go func() {
		tm := <-ch
		if !tm.Equal(now.Add(10 * time.Second)) {
			t.Fatalf("got wrong time")
		}
		atomic.StoreInt32(&ok, 1)
		wg.Done()
	}()

	// Move clock forward to the after channel's time.
	mock.Add(10 * time.Second)
	wg.Wait()
	if atomic.LoadInt32(&ok) == 0 {
		t.Fatal("too late")
	}
}

// Ensure that the mock's After channel doesn't block on write.
func TestMock_UnusedAfter(t *testing.T) {
	mock := NewMock()
	clock := mock.Clock()

	clock.After(1 * time.Millisecond)

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

	mock := NewMock()
	clock := mock.Clock()

	// Execute function after duration.
	clock.AfterFunc(10*time.Second, func() {
		atomic.StoreInt32(&ok, 1)
	})

	// Move clock forward to just before the time.
	mock.Add(9 * time.Second)
	if atomic.LoadInt32(&ok) == 1 {
		t.Fatal("too early")
	}

	// Move clock forward to the after channel's time.
	mock.Add(1 * time.Second)
	if atomic.LoadInt32(&ok) == 0 {
		t.Fatal("too late")
	}
}

func TestMock_Timer(t *testing.T) {
	mock := NewMock()
	clock := mock.Clock()

	mock.Add(time.Hour)

	tmr := clock.Timer(-1 * time.Minute)
	select {
	case <-tmr.C:
		// this is good
	default:
		t.Fatalf("Past timer did not expire")
	}

	tmr = clock.Timer(time.Duration(0))
	select {
	case <-tmr.C:
		// this is good
	default:
		t.Fatalf("Now timer did not expire")
	}

	tmr = clock.Timer(time.Minute)
	select {
	case <-tmr.C:
		t.Fatalf("Future timer did expire")
	default:
	}

	mock.Add(time.Minute)
	select {
	case <-tmr.C:
		// this is good
	default:
		t.Fatalf("Timer did not expire after time moved to its due time")
	}
}

// Ensure that the mock's AfterFunc doesn't execute if stopped.
func TestMock_AfterFunc_Stop(t *testing.T) {
	// Execute function after duration.
	mock := NewMock()
	clock := mock.Clock()
	timer := clock.AfterFunc(10*time.Second, func() {
		t.Fatal("unexpected function execution")
	})

	// Stop timer & move clock forward.
	timer.Stop()
	mock.Add(10 * time.Second)
}

// Ensure that the mock's current time can be changed.
func TestMock_Now(t *testing.T) {
	mock := NewMock()
	clock := mock.Clock()
	if now := clock.Now(); !now.Equal(time.Unix(0, 0)) {
		t.Fatalf("expected epoch, got: %v", now)
	}

	// Add 10 seconds and check the time.
	mock.Add(10 * time.Second)
	if now := clock.Now(); !now.Equal(time.Unix(10, 0)) {
		t.Fatalf("expected epoch, got: %v", now)
	}
}

func TestMock_Since(t *testing.T) {
	mock := NewMock()
	clock := mock.Clock()

	beginning := clock.Now()
	mock.Add(500 * time.Second)
	if since := clock.Since(beginning); since.Seconds() != 500 {
		t.Fatalf("expected 500 since beginning, actually: %v", since.Seconds())
	}
}

// Ensure that the mock can sleep for the correct time.
func TestMock_Sleep(t *testing.T) {
	var ok int32
	var wg sync.WaitGroup

	mock := NewMock()
	clock := mock.Clock()
	wg.Add(1)

	sync := make(chan struct{})
	// This hooks gets into the internal timer that mock Sleep uses
	mock.SetHook(HookBeforeTimer, func(t time.Time) {
		sync <- struct{}{}
	})

	go func() {
		now := clock.Now()
		clock.Sleep(10 * time.Second)
		atomic.StoreInt32(&ok, 1)
		if clock.Now().Before(now.Add(10 * time.Second)) {
			t.Fatal("too early")
		}
		wg.Done()
	}()

	// Wait for us to hit that sleep
	<-sync

	for i := 0; i < 9; i++ {
		mock.Add(time.Second)
		runtime.Gosched()
		if atomic.LoadInt32(&ok) != 0 {
			t.Fatal("too early")
		}
	}
	// Move clock forward to the after the sleep duration.
	mock.Add(time.Second)
	wg.Wait()
	if atomic.LoadInt32(&ok) == 0 {
		t.Fatal("too late")
	}

}

// Ensure that the mock's Tick channel sends at the correct time.
func TestMock_Tick(t *testing.T) {
	var n int32
	mock := NewMock()
	clock := mock.Clock()
	sync := make(chan struct{}, 20)

	// Create a channel to increment every 10 seconds.
	go func() {
		tick := clock.Tick(10 * time.Second)
		sync <- struct{}{}
		for {
			<-tick
			atomic.AddInt32(&n, 1)
			sync <- struct{}{}
		}
	}()

	// Wait for the goroutine to set the timer
	<-sync

	// Move clock forward to just before the first tick.
	mock.Add(9 * time.Second)
	// This bit is unsynchronized but we should never see this fail here
	// the sleep is just to encourage a failure if something is wrong.
	time.Sleep(10 * time.Millisecond)
	if atomic.LoadInt32(&n) != 0 {
		t.Fatalf("expected 0, got %d", n)
	}

	// Move clock forward to the start of the first tick.
	mock.Add(1 * time.Second)
	<-sync
	if atomic.LoadInt32(&n) != 1 {
		t.Fatalf("expected 1, got %d", n)
	}

	// Move clock forward over several ticks.
	mock.Add(30 * time.Second)
	<-sync
	<-sync
	<-sync
	if atomic.LoadInt32(&n) != 4 {
		t.Fatalf("expected 4, got %d", n)
	}
}

// Ensure that the mock's Ticker channel sends at the correct time.
func TestMock_Ticker(t *testing.T) {
	var wg sync.WaitGroup

	mock := NewMock()
	clock := mock.Clock()

	wg.Add(10)
	ticker := clock.Ticker(time.Microsecond)

	// Create a channel to increment every microsecond.
	go func() {
		for {
			<-ticker.C
			wg.Done()
		}
	}()

	// Move clock forward.
	mock.Add(10 * time.Microsecond)
	wg.Wait()
}

// Ensure that the mock's Ticker channel won't block if not read from.
func TestMock_Ticker_Overflow(t *testing.T) {
	mock := NewMock()
	clock := mock.Clock()
	ticker := clock.Ticker(1 * time.Microsecond)
	mock.Add(10 * time.Microsecond)
	ticker.Stop()
}

// Ensure that the mock's Ticker can be stopped.
func TestMock_Ticker_Stop(t *testing.T) {
	mock := NewMock()
	clock := mock.Clock()

	// Create a channel to increment every second.
	ticker := clock.Ticker(1 * time.Second)

	// Move clock forward.
	mock.Add(5 * time.Second)

	ticker.Stop()

	//drain the channel, there must be 5 updates
	var i int
	for len(ticker.C) > 0 {
		<-ticker.C
		i++
	}
	if i != 5 {
		t.Fatalf("Expected 5 updates got %d\n", i)
	}

	// wait for some time in real time for things not to happen
	tmrC := time.After(1 * time.Second)
	// Move clock forward again.
	mock.Add(5 * time.Second)

	select {
	case <-ticker.C:
		t.Fatalf("unexpected tick")
	case <-tmrC:
	}
}

// Ensure that multiple tickers can be used together.
func TestMock_Ticker_Multi(t *testing.T) {
	var wg sync.WaitGroup

	mock := NewMock()
	clock := mock.Clock()

	wg.Add(13)
	ticker1 := clock.Ticker(time.Microsecond)
	ticker3 := clock.Ticker(3 * time.Microsecond)

	// Create a channel to increment every microsecond.
	go func() {
		for {
			select {
			case <-ticker1.C:
				wg.Done()
			case <-ticker3.C:
				wg.Done()
			}
		}
	}()

	// Move clock forward.
	mock.Add(10 * time.Microsecond)
	wg.Wait()
}

func TestHooks(t *testing.T) {
	mock := NewMock()
	clock := mock.Clock()

	executed := []HookPoint{}

	getHook := func(point HookPoint) (HookPoint, Hook) {
		return point, func(time.Time) {
			executed = append(executed, point)
		}
	}

	mock.SetHook(getHook(HookBeforeNow))
	mock.SetHook(getHook(HookAfterNow))
	mock.SetHook(getHook(HookBeforeTimer))
	mock.SetHook(getHook(HookAfterTimer))

	clock.Now()
	clock.Timer(time.Second)

	exp := []HookPoint{
		HookBeforeNow,
		HookAfterNow,
		HookBeforeTimer,
		HookAfterTimer,
	}

	if len(executed) != len(exp) {
		t.Fatalf("expected %+v got %+v", exp, executed)
	}

	for i, v := range exp {
		if executed[i] != v {
			t.Fatalf("expected %+v got %+v", v, executed[i])
		}
	}

	executed = []HookPoint{}
	mock.ResetHooks()
	clock.Now()
	if len(executed) != 0 {
		t.Fatalf("expected no hooks executed, but got %+v", executed)
	}

	p, h := getHook(HookAfterNow)
	mock.SetHookExt(p, "TestHooks", "", h)
	mock.SetHookExt(p, "TestHooks", "blah.go", h)
	mock.SetHookExt(p, "blah", "", h)
	mock.SetHookExt(p, "", "clock_test.go", h)

	clock.Now()

	exp = []HookPoint{
		HookAfterNow,
		HookAfterNow,
	}

	if len(executed) != len(exp) {
		t.Fatalf("expected %+v got %+v", exp, executed)
	}

	for i, v := range exp {
		if executed[i] != v {
			t.Fatalf("expected %+v got %+v", v, executed[i])
		}
	}

	executed = []HookPoint{}
	mock.DelHooks(HookAfterNow, "TestHooks", "")

	clock.Now()

	exp = []HookPoint{
		HookAfterNow,
	}

	if len(executed) != len(exp) {
		t.Fatalf("expected %+v got %+v", exp, executed)
	}

	for i, v := range exp {
		if executed[i] != v {
			t.Fatalf("expected %+v got %+v", v, executed[i])
		}
	}

	executed = []HookPoint{}
	mock.DelHooks(HookAfterNow, "", "")

	clock.Now()

	if len(executed) > 0 {
		t.Fatalf("expected nothing got %+v", executed)
	}

}

func ExampleClock_After() {
	// Create a new mock clock.
	mock := NewMock()
	clock := mock.Clock()
	count := 0

	sync := make(chan struct{})
	ready := make(chan struct{})
	// Create a channel to execute after 10 mock seconds.
	go func() {
		ch := clock.After(10 * time.Second)
		close(ready)
		<-ch
		count = 100
		close(sync)
	}()
	<-ready

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Move the clock forward 5 seconds and print the value again.
	mock.Add(5 * time.Second)
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Move the clock forward 5 seconds to the tick time and check the value.
	mock.Add(5 * time.Second)

	<-sync
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:05 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleClock_AfterFunc() {
	// Create a new mock clock.
	mock := NewMock()
	clock := mock.Clock()
	count := 0
	sync := make(chan struct{})

	// Execute a function after 10 mock seconds.
	clock.AfterFunc(10*time.Second, func() {
		count = 100
		close(sync)
	})

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Move the clock forward 10 seconds and print the new value.
	mock.Add(10 * time.Second)
	<-sync
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleClock_Sleep() {
	// Create a new mock clock.
	mock := NewMock()
	clock := mock.Clock()
	count := 0
	sync := make(chan struct{})
	// This hooks gets into the internal timer that mock Sleep uses
	mock.SetHook(HookBeforeTimer, func(t time.Time) {
		sync <- struct{}{}
	})

	// Execute a function after 10 mock seconds.
	go func() {
		clock.Sleep(10 * time.Second)
		count = 100
		close(sync)
	}()

	// Wait for us to hit that sleep
	<-sync

	// Print the starting value.
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Move the clock forward 10 seconds and print the new value.
	mock.Add(10 * time.Second)
	<-sync
	fmt.Printf("%s: %d\n", clock.Now().UTC(), count)

	// Output:
	// 1970-01-01 00:00:00 +0000 UTC: 0
	// 1970-01-01 00:00:10 +0000 UTC: 100
}

func ExampleClock_Ticker() {
	// Create a new mock clock.
	mock := NewMock()
	clock := mock.Clock()
	count := 0

	ready := make(chan struct{})
	sync := make(chan struct{}, 20)

	// Increment count every mock second.
	go func() {
		ticker := clock.Ticker(1 * time.Second)
		close(ready)
		for {
			<-ticker.C
			count++
			sync <- struct{}{}
		}
	}()
	<-ready

	// Move the clock forward 10 seconds and print the new value.
	mock.Add(10 * time.Second)
	for i := 0; i < 10; i++ {
		<-sync
	}
	fmt.Printf("Count is %d after 10 seconds\n", count)

	// Move the clock forward 5 more seconds and print the new value.
	mock.Add(5 * time.Second)
	for i := 0; i < 5; i++ {
		<-sync
	}
	fmt.Printf("Count is %d after 15 seconds\n", count)

	// Output:
	// Count is 10 after 10 seconds
	// Count is 15 after 15 seconds
}

func ExampleClock_Timer() {
	// Create a new mock clock.
	mock := NewMock()
	clock := mock.Clock()
	count := 0

	ready := make(chan struct{})
	sync := make(chan struct{})
	// Increment count after a mock second.
	go func() {
		timer := clock.Timer(1 * time.Second)
		close(ready)
		<-timer.C
		count++
		close(sync)
	}()
	<-ready

	// Move the clock forward 10 seconds and print the new value.
	mock.Add(10 * time.Second)
	<-sync
	fmt.Printf("Count is %d after 10 seconds\n", count)

	// Output:
	// Count is 1 after 10 seconds
}

func TestMock_TimerMany(t *testing.T) {
	// Create a new mock clock.
	mock := NewMock()
	clock := mock.Clock()

	t1 := clock.Timer(1 * time.Second)
	t2 := clock.Timer(2 * time.Second)

	done1 := false
	done2 := false

	mock.Add(3 * time.Second)

	for !done1 && !done2 {
		select {
		case <-t1.C:
			done1 = true
		case <-t2.C:
			done2 = true
		}
	}
}

// Additional tests suggest by CorgiMan
// https://github.com/benbjohnson/clock/issues/9
func TestRace1(t *testing.T) {
	mock := NewMock()
	go mock.Add(time.Second)
	go mock.Add(time.Second)
}

// TestRace2 is modified from the original so not to blow up like the original one
// but still produces the race if one removes the additional lock
// in Add.  The original test code also passes with -race but fails without -race.
func TestRace2(t *testing.T) {
	mock := NewMock()
	clock := mock.Clock()
	ch := clock.After(1 * time.Second)
	go func() {
		for i := 0; i < 10000; i++ {
			go mock.Add(time.Millisecond * 100)
		}
	}()
	<-ch
}
