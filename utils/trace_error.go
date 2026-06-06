package utils

import (
	"fmt"
	"runtime"
)

type TraceError struct {
	Err  error
	File string
	Line int
}

func (e *TraceError) Error() string {
	return fmt.Sprintf("%s:%d: %v", e.File, e.Line, e.Err)
}

// Wrap creates a TraceError with the current file and line number
func Wrap(err error) *TraceError {
	if err == nil {
		return nil
	}
	// Skip 1 frame to get the caller of Wrap
	_, file, line, ok := runtime.Caller(1)
	if !ok {
		file = "unknown"
		line = 0
	}
	return &TraceError{
		Err:  err,
		File: file,
		Line: line,
	}
}
