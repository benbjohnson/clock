package clock

import (
	"testing"
	"time"
)

type Mock struct {
	UnsynchronizedMock
}

// Mock represents a mock clock that only moves forward programmatically.
// it enforces synchronization before and after advancing the clock to
// ensure that timers are already in place before the clock moves, and that
// timer-related work is done before tests go on to assert the results.
func NewMock(t *testing.T, expectedStarts int) *Mock {
	ret := &Mock{
		UnsynchronizedMock: UnsynchronizedMock{now: time.Unix(0, 0)},
	}
	ExpectUpcomingStarts(expectedStarts).UpcomingEventsOption(&ret.UnsynchronizedMock)
	if t != nil {
		FailOnUnexpectedUpcomingEvent(t).UpcomingEventsOption(&ret.UnsynchronizedMock)
	}
	return ret
}

func (m *Mock) Add(d time.Duration, opts ...Option) {
	opts = append(opts, WaitBefore, WaitAfter)
	m.UnsynchronizedMock.Add(d, opts...)
}

func (m *Mock) Set(t time.Time, opts ...Option) {
	opts = append(opts, WaitBefore, WaitAfter)
	m.UnsynchronizedMock.Set(t, opts...)
}
