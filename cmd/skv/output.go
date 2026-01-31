package main

import (
	"fmt"
	"io"
	"os"
	"sync"
	"time"

	"golang.org/x/term"
)

// Output handles CLI output with verbosity control.
type Output struct {
	out   io.Writer
	err   io.Writer
	quiet bool
	isTTY bool
}

var globalOutput = &Output{
	out:   os.Stdout,
	err:   os.Stderr,
	quiet: false,
	isTTY: term.IsTerminal(int(os.Stdout.Fd())),
}

// SetQuiet enables or disables quiet mode.
func (o *Output) SetQuiet(quiet bool) {
	o.quiet = quiet
}

// IsTTY returns true if stdout is a terminal.
func (o *Output) IsTTY() bool {
	return o.isTTY
}

// Print prints a message unless in quiet mode.
func (o *Output) Print(format string, args ...any) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.out, format+"\n", args...)
}

// Success prints a success message unless in quiet mode.
func (o *Output) Success(format string, args ...any) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.out, format+"\n", args...)
}

// Info prints an informational message unless in quiet mode.
func (o *Output) Info(format string, args ...any) {
	if o.quiet {
		return
	}
	fmt.Fprintf(o.out, format+"\n", args...)
}

// Error prints an error message (always shown).
func (o *Output) Error(format string, args ...any) {
	fmt.Fprintf(o.err, format+"\n", args...)
}

// Spinner provides an animated progress indicator for long-running operations.
type Spinner struct {
	message string
	out     *Output
	stop    chan struct{}
	done    chan struct{}
	mu      sync.Mutex
}

var spinnerFrames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

// StartSpinner creates and starts a new spinner with the given message.
// Returns nil if output is quiet or not a TTY.
func (o *Output) StartSpinner(format string, args ...any) *Spinner {
	if o.quiet || !o.isTTY {
		return nil
	}
	s := &Spinner{
		message: fmt.Sprintf(format, args...),
		out:     o,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
	}
	go s.run()
	return s
}

func (s *Spinner) run() {
	defer close(s.done)
	frame := 0
	ticker := time.NewTicker(80 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stop:
			// Clear the spinner line
			fmt.Fprintf(s.out.out, "\r\033[K")
			return
		case <-ticker.C:
			s.mu.Lock()
			fmt.Fprintf(s.out.out, "\r%s %s", spinnerFrames[frame], s.message)
			s.mu.Unlock()
			frame = (frame + 1) % len(spinnerFrames)
		}
	}
}

// Stop stops the spinner and clears the line.
func (s *Spinner) Stop() {
	if s == nil {
		return
	}
	close(s.stop)
	<-s.done
}
