package engine

import (
	"testing"

	"github.com/aniketpatel/tass/internal/util"
)

func TestEventOrdering(t *testing.T) {
	pq := util.NewPriorityQueue[*Event]()

	seq := int64(0)
	push := func(timeMs float64, typ EventType) {
		pq.Push(&Event{TimeMs: timeMs, Type: typ, SeqNum: seq})
		seq++
	}

	push(100, EventTokenStepDone)
	push(50, EventArrival)
	push(50, EventDispatch)   // Same time as arrival, but later seq
	push(200, EventCompletion)
	push(25, EventArrival)

	// Should come out in time order, FIFO for ties
	expected := []struct {
		timeMs float64
		typ    EventType
	}{
		{25, EventArrival},
		{50, EventArrival},
		{50, EventDispatch},
		{100, EventTokenStepDone},
		{200, EventCompletion},
	}

	for i, exp := range expected {
		if pq.IsEmpty() {
			t.Fatalf("step %d: queue empty, expected event at %.0fms", i, exp.timeMs)
		}
		ev := pq.Pop()
		if ev.TimeMs != exp.timeMs {
			t.Errorf("step %d: time=%.0f, want %.0f", i, ev.TimeMs, exp.timeMs)
		}
		if ev.Type != exp.typ {
			t.Errorf("step %d: type=%s, want %s", i, ev.Type, exp.typ)
		}
	}
}
