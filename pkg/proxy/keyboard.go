package proxy

import (
	"context"
	"os"

	log "github.com/sirupsen/logrus"
	"golang.org/x/term"
)

// KeyboardHandler handles keyboard input and raw mode management
type KeyboardHandler struct {
	ui               *TerminalUI
	hasReceivedEvent *bool
	isConnected      *bool
	showingDetails   *bool
	// Callbacks for actions
	onNavigate     func(direction int)
	onRetry        func()
	onOpen         func()
	onToggleDetails func()
	onQuit         func()
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

// Start begins listening for keyboard input
func (kh *KeyboardHandler) Start(ctx context.Context) {
	// Check if we're in a terminal
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return
	}

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

		// Create a buffered channel for reading stdin
		inputCh := make(chan []byte, 1)

		// Start a separate goroutine to read from stdin
		go func() {
			defer close(inputCh)
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
					case inputCh <- buf[:n]:
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
			case input, ok := <-inputCh:
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
			if kh.onOpen != nil {
				kh.onOpen()
			}
		case 0x64, 0x44: // 'd' or 'D'
			// Toggle alternate screen details view
			if kh.onToggleDetails != nil {
				kh.onToggleDetails()
			}
		}
	}
}
