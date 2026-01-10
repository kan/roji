package proxy

import (
	"sync"
	"time"
)

// RequestLog represents a single request log entry
type RequestLog struct {
	ID        int64     `json:"id"`
	Timestamp time.Time `json:"timestamp"`
	Method    string    `json:"method"`
	Host      string    `json:"host"`
	Path      string    `json:"path"`
	Status    int       `json:"status"`
	Duration  int64     `json:"duration_ms"` // milliseconds
	Service   string    `json:"service"`
	IsMock    bool      `json:"is_mock,omitempty"`
}

// LogBuffer is a thread-safe ring buffer for request logs
type LogBuffer struct {
	mu       sync.RWMutex
	logs     []RequestLog
	capacity int
	nextID   int64

	// Subscribers for real-time log updates
	subMu       sync.RWMutex
	subscribers map[chan RequestLog]struct{}
}

// NewLogBuffer creates a new log buffer with the specified capacity
func NewLogBuffer(capacity int) *LogBuffer {
	return &LogBuffer{
		logs:        make([]RequestLog, 0, capacity),
		capacity:    capacity,
		nextID:      1,
		subscribers: make(map[chan RequestLog]struct{}),
	}
}

// Add adds a new log entry to the buffer
func (lb *LogBuffer) Add(log RequestLog) {
	lb.mu.Lock()
	log.ID = lb.nextID
	lb.nextID++

	if len(lb.logs) >= lb.capacity {
		// Remove oldest entry (shift left)
		copy(lb.logs, lb.logs[1:])
		lb.logs = lb.logs[:len(lb.logs)-1]
	}
	lb.logs = append(lb.logs, log)
	lb.mu.Unlock()

	// Notify subscribers
	lb.notifySubscribers(log)
}

// List returns all logs in the buffer (oldest first)
func (lb *LogBuffer) List() []RequestLog {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	result := make([]RequestLog, len(lb.logs))
	copy(result, lb.logs)
	return result
}

// ListRecent returns the most recent n logs (newest first)
func (lb *LogBuffer) ListRecent(n int) []RequestLog {
	lb.mu.RLock()
	defer lb.mu.RUnlock()

	if n > len(lb.logs) {
		n = len(lb.logs)
	}

	result := make([]RequestLog, n)
	// Reverse order: newest first
	for i := 0; i < n; i++ {
		result[i] = lb.logs[len(lb.logs)-1-i]
	}
	return result
}

// Subscribe creates a channel that receives new log entries
func (lb *LogBuffer) Subscribe() (chan RequestLog, func()) {
	ch := make(chan RequestLog, 10) // Buffer to avoid blocking

	lb.subMu.Lock()
	lb.subscribers[ch] = struct{}{}
	lb.subMu.Unlock()

	cleanup := func() {
		lb.subMu.Lock()
		delete(lb.subscribers, ch)
		lb.subMu.Unlock()
		close(ch)
	}

	return ch, cleanup
}

// notifySubscribers sends a log entry to all subscribers
func (lb *LogBuffer) notifySubscribers(log RequestLog) {
	lb.subMu.RLock()
	defer lb.subMu.RUnlock()

	for ch := range lb.subscribers {
		select {
		case ch <- log:
		default:
			// Channel full, skip (non-blocking)
		}
	}
}

// SubscriberCount returns the number of active subscribers
func (lb *LogBuffer) SubscriberCount() int {
	lb.subMu.RLock()
	defer lb.subMu.RUnlock()
	return len(lb.subscribers)
}

// Clear removes all logs from the buffer
func (lb *LogBuffer) Clear() {
	lb.mu.Lock()
	defer lb.mu.Unlock()
	lb.logs = lb.logs[:0]
}
