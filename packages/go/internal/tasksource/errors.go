package tasksource

import "errors"

var (
	// ErrTaskNotFound is returned when a task cannot be found.
	ErrTaskNotFound = errors.New("task not found")

	// ErrSourceNotFound is returned when a source cannot be found.
	ErrSourceNotFound = errors.New("source not found")

	// ErrInvalidConfig is returned when source configuration is invalid.
	ErrInvalidConfig = errors.New("invalid source configuration")

	// ErrReadOnly is returned when attempting to modify a read-only source.
	ErrReadOnly = errors.New("source is read-only")

	// ErrNotSupported is returned when an operation is not supported by a source.
	ErrNotSupported = errors.New("operation not supported")
)
