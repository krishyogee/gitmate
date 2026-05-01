package tui

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

const spinnerFrames = `⠋⠙⠹⠸⠼⠴⠦⠧⠇⠏`

type Stream struct {
	out       io.Writer
	mu        sync.Mutex
	current   string
	stop      chan struct{}
	stopped   chan struct{}
	enabled   bool
	frameIdx  int
}

func NewStream() *Stream {
	return &Stream{out: os.Stdout, enabled: IsTTY()}
}

func (s *Stream) Start(label string) {
	s.mu.Lock()
	if s.enabled && s.stop != nil {
		s.mu.Unlock()
		s.Update(label)
		return
	}
	s.current = label
	s.stop = make(chan struct{})
	s.stopped = make(chan struct{})
	s.mu.Unlock()

	if !s.enabled {
		fmt.Fprintf(s.out, "%s %s\n", Subtle.Render("…"), label)
		return
	}

	go s.loop()
}

func (s *Stream) loop() {
	t := time.NewTicker(80 * time.Millisecond)
	defer t.Stop()
	frames := []rune(spinnerFrames)
	for {
		select {
		case <-s.stop:
			close(s.stopped)
			return
		case <-t.C:
			s.mu.Lock()
			s.frameIdx = (s.frameIdx + 1) % len(frames)
			frame := string(frames[s.frameIdx])
			line := fmt.Sprintf("\r%s %s ", Cmd.Render(frame), s.current)
			fmt.Fprint(s.out, line)
			s.mu.Unlock()
		}
	}
}

func (s *Stream) Update(label string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.current = label
}

func (s *Stream) Done(label string) {
	s.finish(OK.Render("✓"), label)
}

func (s *Stream) Fail(label string) {
	s.finish(Err.Render("✗"), label)
}

func (s *Stream) Info(label string) {
	s.finish(Subtle.Render("·"), label)
}

func (s *Stream) finish(icon, label string) {
	s.mu.Lock()
	wasRunning := s.enabled && s.stop != nil
	stop := s.stop
	stopped := s.stopped
	s.stop = nil
	s.stopped = nil
	s.mu.Unlock()

	if wasRunning {
		close(stop)
		<-stopped
		fmt.Fprint(s.out, "\r"+strings.Repeat(" ", 80)+"\r")
	}
	fmt.Fprintf(s.out, "%s %s\n", icon, label)
}

func (s *Stream) Println(line string) {
	s.mu.Lock()
	wasRunning := s.enabled && s.stop != nil
	s.mu.Unlock()
	if wasRunning {
		fmt.Fprint(s.out, "\r"+strings.Repeat(" ", 80)+"\r")
	}
	fmt.Fprintln(s.out, line)
}
