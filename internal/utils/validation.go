package utils

import (
	"gpt-load/internal/models"
)

// IsValidKeyStatus checks if the given status is a valid key status.
func IsValidKeyStatus(status string) bool {
	return status == "" || status == models.KeyStatusActive || status == models.KeyStatusInvalid
}

// ValidateKeyStatus returns an error if the status is invalid.
func ValidateKeyStatus(status string) error {
	if !IsValidKeyStatus(status) {
		return &ValidationError{Field: "status", Message: "invalid status value"}
	}
	return nil
}

// ValidationError represents a validation error.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return e.Field + ": " + e.Message
}
