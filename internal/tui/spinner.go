// SPDX-License-Identifier: GPL-3.0-or-later
// Copyright 2026 Benjamin Bellamy

package tui

import (
	"fmt"
	"os"
	"sync"
	"time"
)

// Spinner is a tiny single-line progress indicator that updates a fixed line
// on stderr (via carriage-return overwrite). On non-TTY output it's a silent
// no-op, so logs/pipes stay clean.
//
// Usage:
//
//	sp := tui.NewSpinner("scanning apt")
//	sp.Start()
//	defer sp.Stop()
//	// … long-running work …
type Spinner struct {
	label  string
	frames []string
	stop   chan struct{}
	wg     sync.WaitGroup
	active bool
	mu     sync.Mutex
}

// NewSpinner builds a spinner with the default Braille frame set.
func NewSpinner(label string) *Spinner {
	return &Spinner{
		label:  label,
		frames: []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"},
		stop:   make(chan struct{}),
	}
}

// SetLabel updates the visible label without restarting the spinner.
func (s *Spinner) SetLabel(label string) {
	s.mu.Lock()
	s.label = label
	s.mu.Unlock()
}

// Start begins the spin. Safe to call when stderr isn't a TTY (becomes a no-op).
func (s *Spinner) Start() {
	if !isTTY(os.Stderr) {
		return
	}
	s.active = true
	s.wg.Add(1)
	go s.loop()
}

// Stop ends the spin, clears the line, and blocks until the goroutine exits.
func (s *Spinner) Stop() {
	if !s.active {
		return
	}
	close(s.stop)
	s.wg.Wait()
	s.active = false
}

func (s *Spinner) loop() {
	defer s.wg.Done()
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	i := 0
	theme := DefaultTheme()
	for {
		select {
		case <-s.stop:
			// Clear the spinner line.
			fmt.Fprint(os.Stderr, "\r\033[K")
			return
		case <-t.C:
			s.mu.Lock()
			label := s.label
			s.mu.Unlock()
			frame := theme.Title.Render(s.frames[i])
			fmt.Fprintf(os.Stderr, "\r  %s %s", frame, theme.Subtitle.Render(label))
			i = (i + 1) % len(s.frames)
		}
	}
}
