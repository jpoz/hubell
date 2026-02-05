package notify

import (
	"fmt"
	"os"
)

// SendDesktopNotification sends a desktop notification using OSC 777 escape sequences
// Format: \033]777;notify;<title>;<body>\007
// If running in tmux, wraps with tmux escape sequences:
// \033Ptmux;\033\033]777;notify;<title>;<body>\007\033\\
func SendDesktopNotification(title, body string) {
	var escape string

	// Check if we're inside tmux
	if os.Getenv("TMUX") != "" {
		// Tmux requires wrapping: \033Ptmux;\033<OSC_CODE>\033\\
		escape = fmt.Sprintf("\033Ptmux;\033\033]777;notify;%s;%s\007\033\\", title, body)
	} else {
		// Standard OSC 777
		escape = fmt.Sprintf("\033]777;notify;%s;%s\007", title, body)
	}

	// Write to /dev/tty to ensure it reaches the terminal
	tty, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		// Fallback to stdout if /dev/tty not available
		os.Stdout.Write([]byte(escape))
		return
	}
	defer tty.Close()

	tty.Write([]byte(escape))
}
