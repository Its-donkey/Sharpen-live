package logging

import "sync"

const maxStoredLogEntries = 300

var logStream = newLogStream()

// Subscribe returns a channel of log entries and a snapshot of current logs.
func Subscribe() (chan []byte, [][]byte) { return logStream.Subscribe() }

// Unsubscribe removes a previously subscribed channel.
func Unsubscribe(ch chan []byte) { logStream.Unsubscribe(ch) }

type logStreamState struct {
	mu          sync.RWMutex
	buffer      [][]byte
	subscribers map[chan []byte]struct{}
}

func newLogStream() *logStreamState {
	return &logStreamState{
		buffer:      make([][]byte, 0, maxStoredLogEntries),
		subscribers: make(map[chan []byte]struct{}),
	}
}

func (l *logStreamState) Broadcast(entry []byte) {
	if l == nil || entry == nil {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()

	if len(l.buffer) >= maxStoredLogEntries {
		copy(l.buffer, l.buffer[1:])
		l.buffer[len(l.buffer)-1] = append([]byte(nil), entry...)
	} else {
		l.buffer = append(l.buffer, append([]byte(nil), entry...))
	}
	for ch := range l.subscribers {
		select {
		case ch <- entry:
		default:
		}
	}
}

func (l *logStreamState) Subscribe() (chan []byte, [][]byte) {
	ch := make(chan []byte, 64)
	l.mu.RLock()
	snapshot := make([][]byte, len(l.buffer))
	for i := range l.buffer {
		snapshot[i] = append([]byte(nil), l.buffer[i]...)
	}
	l.mu.RUnlock()

	l.mu.Lock()
	l.subscribers[ch] = struct{}{}
	l.mu.Unlock()
	return ch, snapshot
}

func (l *logStreamState) Unsubscribe(ch chan []byte) {
	if l == nil || ch == nil {
		return
	}
	l.mu.Lock()
	if _, ok := l.subscribers[ch]; ok {
		delete(l.subscribers, ch)
		close(ch)
	}
	l.mu.Unlock()
}
