package proxy

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"

	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// KeyboardHandler handles keyboard input and raw mode management
type KeyboardHandler struct {
	ui               *TerminalUI
	hasReceivedEvent *bool
	isConnected      *bool
	showingDetails   *bool
	paused           bool // Flag to pause input processing
	pauseMutex       sync.Mutex
	inputCh          chan []byte // Channel for buffered keyboard input
	// Callbacks for actions
	onNavigate      func(direction int)
	onRetry         func()
	onOpen          func()
	onToggleDetails func()
	onQuit          func()
}

// NewKeyboardHandler creates a new KeyboardHandler instance
func NewKeyboardHandler(ui *TerminalUI, hasReceivedEvent *bool, isConnected *bool, showingDetails *bool) *KeyboardHandler {
	return &KeyboardHandler{
		ui:              ui,
		hasReceivedEvent: hasReceivedEvent,
		isConnected:     isConnected,
		showingDetails:  showingDetails,
	}
}

// SetCallbacks sets the action callbacks
func (kh *KeyboardHandler) SetCallbacks(
	onNavigate func(direction int),
	onRetry func(),
	onOpen func(),
	onToggleDetails func(),
	onQuit func(),
) {
	kh.onNavigate = onNavigate
	kh.onRetry = onRetry
	kh.onOpen = onOpen
	kh.onToggleDetails = onToggleDetails
	kh.onQuit = onQuit
}

// Pause temporarily stops processing keyboard input (while less is running)
func (kh *KeyboardHandler) Pause() {
	kh.pauseMutex.Lock()
	defer kh.pauseMutex.Unlock()
	kh.paused = true
	log.WithField("prefix", "KeyboardHandler.Pause").Debug("Keyboard input paused")
}

// Resume resumes processing keyboard input (after less exits)
func (kh *KeyboardHandler) Resume() {
	kh.pauseMutex.Lock()
	defer kh.pauseMutex.Unlock()
	kh.paused = false
	log.WithField("prefix", "KeyboardHandler.Resume").Debug("Keyboard input resumed")
}

// DrainBufferedInput discards any input that was buffered while paused
// This should be called after less exits but before Resume() to prevent
// keypresses meant for less from being processed by the app
func (kh *KeyboardHandler) DrainBufferedInput() {
	if kh.inputCh == nil {
		return
	}
	// Drain the channel non-blockingly
	drained := 0
	for {
		select {
		case <-kh.inputCh:
			drained++
		default:
			// Channel is empty
			if drained > 0 {
				log.WithField("prefix", "KeyboardHandler.DrainBufferedInput").Debugf("Drained %d buffered inputs", drained)
			}
			return
		}
	}
}

// Start begins listening for keyboard input and terminal resize signals
func (kh *KeyboardHandler) Start(ctx context.Context) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

	// Set up terminal resize signal handler
	sigwinchCh := make(chan os.Signal, 1)
	signal.Notify(sigwinchCh, syscall.SIGWINCH)

	// Start goroutine to handle terminal resize signals
	go func() {
		for {
			select {
			case <-ctx.Done():
				signal.Stop(sigwinchCh)
				close(sigwinchCh)
				return
			case <-sigwinchCh:
				// Terminal was resized - trigger a redraw with new dimensions
				log.WithField("prefix", "KeyboardHandler.Start").Debug("Terminal resize detected")
				kh.ui.HandleResize(*kh.hasReceivedEvent)
			}
		}
	}()

	go func() {
		// Enter raw mode once and keep it
		oldState, err := term.MakeRaw(int(os.Stdin.Fd()))
		if err != nil {
			return
		}

		// Store the raw mode state for use in UI rendering
		kh.ui.SetRawModeState(oldState)

		// Ensure we restore terminal state when this goroutine exits
		defer term.Restore(int(os.Stdin.Fd()), oldState)

		// Create a buffered channel for reading stdin and store it as a field
		kh.inputCh = make(chan []byte, 1)

		// Start a separate goroutine to read from stdin
		go func() {
			defer close(kh.inputCh)
			buf := make([]byte, 3) // Buffer for escape sequences
			for {
				select {
				case <-ctx.Done():
					return
				default:
					n, err := os.Stdin.Read(buf)
					if err != nil {
						// Log the error but don't crash the application
						log.WithField("prefix", "proxy.KeyboardHandler.Start").Debugf("Error reading stdin: %v", err)
						return
					}
					if n == 0 {
						continue
					}
					select {
					case kh.inputCh <- buf[:n]:
					case <-ctx.Done():
						return
					}
				}
			}
		}()

		// Main loop to process keyboard input
		for {
			select {
			case <-ctx.Done():
				return
			case input, ok := <-kh.inputCh:
				if !ok {
					return
				}

				// Process the input
				kh.processInput(input)
			}
		}
	}()
}

// processInput handles keyboard input including arrow keys
func (kh *KeyboardHandler) processInput(input []byte) {
	if len(input) == 0 {
		return
	}

	// Check if input processing is paused (e.g., while less is running)
	kh.pauseMutex.Lock()
	paused := kh.paused
	kh.pauseMutex.Unlock()

	if paused {
		// Discard all input while paused
		log.WithField("prefix", "KeyboardHandler.processInput").Debugf("Discarding input while paused: %v", input)
		return
	}

	// Handle single character keys
	if len(input) == 1 {
		switch input[0] {
		case 0x03: // Ctrl+C
			if kh.onQuit != nil {
				kh.onQuit()
			}
			return
		}
	}

	// Disable all other shortcuts until first event is received or while not connected
	if !*kh.hasReceivedEvent || !*kh.isConnected {
		return
	}

	// Handle escape sequences (arrow keys)
	if len(input) == 3 && input[0] == 0x1B && input[1] == 0x5B {
		// Disable navigation while in details view
		if *kh.showingDetails {
			return
		}

		switch input[2] {
		case 0x41: // Up arrow
			if kh.onNavigate != nil {
				kh.onNavigate(-1)
			}
		case 0x42: // Down arrow
			if kh.onNavigate != nil {
				kh.onNavigate(1)
			}
		}
		return
	}

	// Handle single character keys (after quit/ctrl+c check)
	if len(input) == 1 {
		switch input[0] {
		case 0x72, 0x52: // 'r' or 'R'
			if !*kh.showingDetails && kh.onRetry != nil {
				kh.onRetry()
			}
		case 0x6F, 0x4F: // 'o' or 'O'
			if !*kh.showingDetails && kh.onOpen != nil {
				kh.onOpen()
			}
		case 0x64, 0x44: // 'd' or 'D'
			// Toggle alternate screen details view (but not while already showing)
			if !*kh.showingDetails && kh.onToggleDetails != nil {
				kh.onToggleDetails()
			}
		}
	}
}
