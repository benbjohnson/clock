package clock

import (
	"fmt"
	"os"
	"sync"
	"testing"
	"time"
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
		if !ok {
			t.Fatal("too late")
		}
	}()
	gosched()

	<-New().After(20 * time.Millisecond)
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
		if !ok {
			t.Fatal("too late")
		}
	}()
	gosched()

	var wg sync.WaitGroup
	wg.Add(1)
	New().AfterFunc(20*time.Millisecond, func() {
		wg.Done()
	})
	wg.Wait()
	if !ok {
		t.Fatal("too early")
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
	var ok bool
	go func() {
		time.Sleep(10 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		if !ok {
			t.Fatal("too late")
		}
	}()
	gosched()

	New().Sleep(20 * time.Millisecond)
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
		if !ok {
			t.Fatal("too late")
		}
	}()
	gosched()

	c := New().Tick(20 * time.Millisecond)
	<-c
	<-c
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure that the clock's ticker ticks correctly.
func TestClock_Ticker(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(100 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(200 * time.Millisecond)
		if !ok {
			t.Fatal("too late")
		}
	}()
	gosched()

	ticker := New().NewTicker(50 * time.Millisecond)
	<-ticker.C
	<-ticker.C
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure that the clock's ticker can stop correctly.
func TestClock_Ticker_Stp(t *testing.T) {
	ticker := New().NewTicker(20 * time.Millisecond)
	<-ticker.C
	ticker.Stop()
	select {
	case <-ticker.C:
		t.Fatal("unexpected send")
	case <-time.After(30 * time.Millisecond):
	}
}

// Ensure that the clock's ticker can reset correctly.
func TestClock_Ticker_Rst(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(30 * time.Millisecond)
		ok = true
	}()
	gosched()

	ticker := New().NewTicker(20 * time.Millisecond)
	<-ticker.C
	ticker.Reset(5 * time.Millisecond)
	<-ticker.C
	if ok {
		t.Fatal("too late")
	}
	ticker.Stop()
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
		if !ok {
			t.Fatal("too late")
		}
	}()
	gosched()

	timer := New().NewTimer(20 * time.Millisecond)
	<-timer.C
	if !ok {
		t.Fatal("too early")
	}

	if timer.Stop() {
		t.Fatal("timer still running")
	}
}

// Ensure that the clock's timer can be stopped.
func TestClock_Timer_Stop(t *testing.T) {
	timer := New().NewTimer(20 * time.Millisecond)
	if !timer.Stop() {
		t.Fatal("timer not running")
	}
	if timer.Stop() {
		t.Fatal("timer wasn't cancelled")
	}
	select {
	case <-timer.C:
		t.Fatal("unexpected send")
	case <-time.After(30 * time.Millisecond):
	}
}

// Ensure that the clock's timer can be reset.
func TestClock_Timer_Reset(t *testing.T) {
	var ok bool
	go func() {
		time.Sleep(20 * time.Millisecond)
		ok = true
	}()
	go func() {
		time.Sleep(30 * time.Millisecond)
		if !ok {
			t.Fatal("too late")
		}
	}()
	gosched()

	timer := New().NewTimer(10 * time.Millisecond)
	if !timer.Reset(20 * time.Millisecond) {
		t.Fatal("timer not running")
	}

	<-timer.C
	if !ok {
		t.Fatal("too early")
	}
}

// Ensure reset can be called immediately after reading channel
func TestClock_Timer_Reset_Unlock(t *testing.T) {
	clock := NewUnsynchronizedMock()
	timer := clock.NewTimer(1 * time.Second)

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()

		<-timer.C
		timer.Reset(1 * time.Second)

		<-timer.C
	}()

	clock.Add(2 * time.Second)
	wg.Wait()
}

func warn(v ...interface{})              { fmt.Fprintln(os.Stderr, v...) }
func warnf(msg string, v ...interface{}) { fmt.Fprintf(os.Stderr, msg+"\n", v...) }
