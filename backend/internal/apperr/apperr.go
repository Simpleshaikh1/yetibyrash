// Package apperr holds error types shared by the shop's services and the HTTP layer.
package apperr

import "errors"

// Invalid is a problem with something the user typed. Message is shown to them as written;
// Field names the form field it's about.
type Invalid struct {
	Field   string
	Message string
}

func (e *Invalid) Error() string { return e.Field + ": " + e.Message }

func NewInvalid(field, message string) error { return &Invalid{Field: field, Message: message} }

// ErrNotFound: the thing asked for doesn't exist (or the caller may not see it).
var ErrNotFound = errors.New("not found")

// Conflict is a request that can't be done in the current state; Message explains why.
type Conflict struct {
	Code    string
	Message string
}

func (e *Conflict) Error() string { return e.Code + ": " + e.Message }
