package errors

import (
	"fmt"
	"runtime"
	"strings"
)

// Error wraps an error with a message and stack trace.
type Error struct {
	msg   string
	err   error
	stack string
}

func (e *Error) Error() string {
	if e.err == nil {
		return e.msg
	}
	return fmt.Sprintf("%s: %v", e.msg, e.err)
}

func (e *Error) Unwrap() error {
	return e.err
}

func (e *Error) StackTrace() string {
	return e.stack
}

// Wrap wraps err with msg and stack trace.
func Wrap(err error, msg string) error {
	if err == nil {
		return nil
	}
	return &Error{
		msg:   msg,
		err:   err,
		stack: callers(),
	}
}

// New creates a new error with stack trace.
func New(msg string) error {
	return &Error{
		msg:   msg,
		stack: callers(),
	}
}

// callers returns a formatted stack trace.
func callers() string {
	pcs := make([]uintptr, 16)
	n := runtime.Callers(3, pcs)
	frames := runtime.CallersFrames(pcs[:n])
	var b strings.Builder
	for {
		frame, more := frames.Next()
		b.WriteString(fmt.Sprintf("%s\n\t%s:%d\n", frame.Function, frame.File, frame.Line))
		if !more {
			break
		}
	}
	return b.String()
}
