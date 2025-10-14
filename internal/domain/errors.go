package domain

import "fmt"

type ValidationError struct {
	Field   string
	Message string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("validation error on field '%s': %s", e.Field, e.Message)
}

// NotFoundError represents not found error
type NotFoundError struct {
	Entity string
	ID     string
}

func (e NotFoundError) Error() string {
	return fmt.Sprintf("%s with ID '%s' not found", e.Entity, e.ID)
}

// ConflictError represents conflict error
type ConflictError struct {
	Entity string
	Reason string
}

func (e ConflictError) Error() string {
	return fmt.Sprintf("%s conflict: %s", e.Entity, e.Reason)
}

// NewValidationError creates new validation error
func NewValidationError(field, message string) error {
	return ValidationError{
		Field:   field,
		Message: message,
	}
}

// NewNotFoundError creates new not found error
func NewNotFoundError(entity, id string) error {
	return NotFoundError{
		Entity: entity,
		ID:     id,
	}
}

// NewConflictError creates new conflict error
func NewConflictError(entity, reason string) error {
	return ConflictError{
		Entity: entity,
		Reason: reason,
	}
}
