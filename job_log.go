package main

import (
	"fmt"
	"os"
	"sync"
)

func (s *sessionState) Logf(format string, args ...interface{}) {
	if s == nil {
		fmt.Printf(format, args...)
		return
	}
	s.LogMu.Lock()
	defer s.LogMu.Unlock()
	fmt.Fprintf(&s.Log, format, args...)
}

func (s *sessionState) Logln(args ...interface{}) {
	if s == nil {
		fmt.Println(args...)
		return
	}
	s.LogMu.Lock()
	defer s.LogMu.Unlock()
	fmt.Fprintln(&s.Log, args...)
}

func (s *sessionState) FlushLog() {
	if s == nil {
		return
	}
	s.LogMu.Lock()
	text := s.Log.String()
	s.Log.Reset()
	s.LogMu.Unlock()
	if text == "" {
		return
	}
	consoleMu.Lock()
	defer consoleMu.Unlock()
	_, _ = fmt.Fprint(os.Stdout, text)
}

var consoleMu sync.Mutex
