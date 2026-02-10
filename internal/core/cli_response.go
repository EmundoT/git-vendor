package core

import (
	"encoding/json"
	"fmt"
	"os"
)

// CLIResponse is the structured JSON output for LLM-friendly CLI commands (Spec 072).
// All new commands use this format. Existing commands retain their JSONOutput format for backward compatibility.
//
// Schema:
//
//	{
//	  "success": true|false,
//	  "data": { ... },          // Command-specific payload (omitted on error)
//	  "error": {                 // Present only on failure
//	    "code": "VENDOR_NOT_FOUND",
//	    "message": "Human-readable description"
//	  }
//	}
type CLIResponse struct {
	Success bool            `json:"success"`
	Data    interface{}     `json:"data,omitempty"`
	Error   *CLIErrorDetail `json:"error,omitempty"`
}

// CLIErrorDetail contains machine-readable error code and human-readable message.
type CLIErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// CLI exit codes for LLM-friendly commands (Spec 072).
const (
	ExitSuccess          = 0
	ExitGeneralError     = 1
	ExitVendorNotFound   = 2
	ExitInvalidArguments = 3
	ExitValidationFailed = 4
	ExitNetworkError     = 5
)

// CLI error codes for structured JSON error responses.
const (
	ErrCodeVendorNotFound   = "VENDOR_NOT_FOUND"
	ErrCodeVendorExists     = "VENDOR_EXISTS"
	ErrCodeMappingNotFound  = "MAPPING_NOT_FOUND"
	ErrCodeMappingExists    = "MAPPING_EXISTS"
	ErrCodeInvalidArguments = "INVALID_ARGUMENTS"
	ErrCodeNotInitialized   = "NOT_INITIALIZED"
	ErrCodeConfigError      = "CONFIG_ERROR"
	ErrCodeValidationFailed = "VALIDATION_FAILED"
	ErrCodeNetworkError     = "NETWORK_ERROR"
	ErrCodeInternalError    = "INTERNAL_ERROR"
	ErrCodeRefNotFound      = "REF_NOT_FOUND"
	ErrCodeInvalidKey       = "INVALID_KEY"
)

// EmitCLISuccess writes a successful CLIResponse as JSON to stdout and exits with code 0.
func EmitCLISuccess(data interface{}) {
	resp := CLIResponse{Success: true, Data: data}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp) //nolint:errcheck
}

// EmitCLIError writes an error CLIResponse as JSON to stdout and exits with the given code.
// Returns the exit code for the caller to use with os.Exit.
func EmitCLIError(code string, message string, exitCode int) int {
	resp := CLIResponse{
		Success: false,
		Error:   &CLIErrorDetail{Code: code, Message: message},
	}
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	_ = enc.Encode(resp) //nolint:errcheck
	return exitCode
}

// CLIExitCodeForError maps structured error types to CLI exit codes.
func CLIExitCodeForError(err error) int {
	switch {
	case IsVendorNotFound(err):
		return ExitVendorNotFound
	case IsValidationError(err):
		return ExitValidationFailed
	default:
		return ExitGeneralError
	}
}

// CLIErrorCodeForError maps structured error types to CLI error code strings.
func CLIErrorCodeForError(err error) string {
	switch {
	case IsVendorNotFound(err):
		return ErrCodeVendorNotFound
	case IsValidationError(err):
		return ErrCodeValidationFailed
	default:
		return ErrCodeInternalError
	}
}

// FormatCLIMessage formats a simple text message for non-JSON CLI output.
func FormatCLIMessage(format string, args ...interface{}) {
	fmt.Printf(format+"\n", args...)
}
