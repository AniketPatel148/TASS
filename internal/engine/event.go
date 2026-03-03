// Package engine implements the discrete-event simulation engine.
package engine

import "fmt"

// EventType identifies the kind of simulation event.
type EventType int

const (
	EventArrival        EventType = iota // New request arrives
	EventDispatch                        // Worker becomes free, dispatch next batch
	EventTokenStepDone                   // One autoregressive decode step completed
	EventCompletion                      // Request has generated all tokens
	EventPeriodicSample                  // Periodic metrics snapshot
)

func (et EventType) String() string {
	switch et {
	case EventArrival:
		return "Arrival"
	case EventDispatch:
		return "Dispatch"
	case EventTokenStepDone:
		return "TokenStepDone"
	case EventCompletion:
		return "Completion"
	case EventPeriodicSample:
		return "PeriodicSample"
	default:
		return fmt.Sprintf("EventType(%d)", int(et))
	}
}

// Event is a single simulation event scheduled at a specific time.
type Event struct {
	TimeMs    float64   // When this event fires (simulation clock)
	Type      EventType // What kind of event
	RequestID int       // Associated request ID (-1 if not applicable)
	WorkerID  int       // Associated worker ID (-1 if not applicable)
	SeqNum    int64     // Tie-breaking sequence number (FIFO among same-time events)
}

// Priority implements util.HeapItem. Events are ordered by time, then sequence number.
func (e *Event) Priority() float64 {
	// Use time as primary priority. SeqNum provides FIFO ordering for ties.
	// We encode seqnum as a tiny fraction to break ties without affecting ordering.
	return e.TimeMs + float64(e.SeqNum)*1e-15
}
