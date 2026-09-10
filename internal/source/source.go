// Package source reports a failure of the stream a decoder reads from.
//
// One rule is shared by the three packages that read a stream: a failure of
// the source is never the end of the log. A cause whose chain holds io.EOF — a
// torn upload, a closed transport — would otherwise satisfy
// errors.Is(err, io.EOF), and every caller whose loop breaks on the clean end
// of a log would read a broken stream as a complete run.
//
// It is internal because it is not part of the public API: a consumer sees the
// errors it builds, never how they are built.
package source

import (
	"errors"
	"fmt"
	"io"
)

// Failed reports cause, a failure of the source, prefixed with msg. The cause
// stays reachable through errors.Is and errors.As, with one exception:
// errors.Is(err, io.EOF) is false whatever the cause's chain holds.
func Failed(msg string, cause error) error {
	if !errors.Is(cause, io.EOF) {
		return fmt.Errorf("%s: %w", msg, cause)
	}

	return &eofHiddenError{msg: msg + ": " + cause.Error(), cause: cause}
}

// eofHiddenError is a failure whose cause's chain holds io.EOF. An error chain
// cannot lose one link, so it has no Unwrap for errors.Is to walk into;
// it answers Is and As for the cause instead, and says no to io.EOF alone.
type eofHiddenError struct {
	msg   string
	cause error
}

func (e *eofHiddenError) Error() string { return e.msg }

// Is reports whether target is in the cause's chain, unless target is io.EOF.
func (e *eofHiddenError) Is(target error) bool {
	return target != io.EOF && errors.Is(e.cause, target)
}

// As finds the first error in the cause's chain that matches target.
func (e *eofHiddenError) As(target any) bool { return errors.As(e.cause, target) }
