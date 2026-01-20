package keylistener

import (
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

const (
	KeyEscape    = 27
	KeyCtrlC     = 3
	KeyBackspace = 127
	KeyEnter     = 13
)

type KeyEvent struct {
	Key  byte
	Rune rune
}

type Listener struct {
	stopCh   chan struct{}
	doneCh   chan struct{} // signals readLoop has exited
	eventCh  chan KeyEvent
	inputBuf []rune
	bufMu    sync.Mutex
	oldState *term.State
	active   bool
	mu       sync.Mutex
}

func New() *Listener {
	return &Listener{
		stopCh:   make(chan struct{}),
		eventCh:  make(chan KeyEvent, 10),
		inputBuf: make([]rune, 0, 256),
	}
}

// Start begins listening for key events in raw terminal mode
func (l *Listener) Start() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.active {
		return nil
	}

	// Wait for any previous readLoop to finish (with timeout)
	if l.doneCh != nil {
		select {
		case <-l.doneCh:
			// Previous goroutine finished
		default:
			// Previous goroutine still running - wait briefly
			l.mu.Unlock()
			select {
			case <-l.doneCh:
			case <-time.After(100 * time.Millisecond):
				// Timeout - proceed anyway but may have zombie goroutine
			}
			l.mu.Lock()
		}
	}

	// Put terminal in raw mode
	oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	l.oldState = oldState
	l.active = true
	l.stopCh = make(chan struct{})
	l.doneCh = make(chan struct{})
	l.eventCh = make(chan KeyEvent, 10)

	go l.readLoop()
	return nil
}

// Stop restores terminal state and stops listening
func (l *Listener) Stop() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if !l.active {
		return
	}

	close(l.stopCh)
	if l.oldState != nil {
		term.Restore(int(os.Stdin.Fd()), l.oldState)
		l.oldState = nil
	}
	l.active = false
}

func (l *Listener) readLoop() {
	defer func() {
		// Signal that readLoop has exited
		if l.doneCh != nil {
			close(l.doneCh)
		}
	}()

	buf := make([]byte, 1)
	for {
		select {
		case <-l.stopCh:
			return
		default:
			n, err := os.Stdin.Read(buf)
			if err != nil || n == 0 {
				// Check if we should stop
				select {
				case <-l.stopCh:
					return
				default:
					continue
				}
			}

			key := buf[0]
			event := KeyEvent{Key: key, Rune: rune(key)}

			// Send to event channel (non-blocking)
			select {
			case l.eventCh <- event:
			default:
			}

			// Buffer printable characters for follow-up input
			l.bufMu.Lock()
			if key == KeyBackspace {
				// Handle backspace
				if len(l.inputBuf) > 0 {
					l.inputBuf = l.inputBuf[:len(l.inputBuf)-1]
				}
			} else if key != KeyEscape && key != KeyCtrlC && key >= 32 && key < 127 {
				l.inputBuf = append(l.inputBuf, rune(key))
			}
			l.bufMu.Unlock()
		}
	}
}

// Events returns the channel for key events
func (l *Listener) Events() <-chan KeyEvent {
	return l.eventCh
}

// GetBufferedInput returns and clears the buffered follow-up input
func (l *Listener) GetBufferedInput() string {
	l.bufMu.Lock()
	defer l.bufMu.Unlock()

	result := string(l.inputBuf)
	l.inputBuf = l.inputBuf[:0]
	return result
}

// ClearBuffer discards any buffered input
func (l *Listener) ClearBuffer() {
	l.bufMu.Lock()
	defer l.bufMu.Unlock()
	l.inputBuf = l.inputBuf[:0]
}

// IsActive returns whether the listener is currently active
func (l *Listener) IsActive() bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.active
}
