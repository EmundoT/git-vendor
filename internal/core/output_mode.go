package core

// OutputMode controls how output is displayed
type OutputMode int

// OutputMode constants define available output formatting modes.
const (
	OutputNormal OutputMode = iota // Default: styled output
	OutputQuiet                    // Minimal output
	OutputJSON                     // Structured JSON
)

// NonInteractiveFlags groups all non-interactive options
type NonInteractiveFlags struct {
	Yes  bool       // Auto-approve prompts
	Mode OutputMode // Output formatting mode
}

// JSONOutput represents structured output
type JSONOutput struct {
	Status  string                 `json:"status"`            // "success", "error", "warning"
	Message string                 `json:"message,omitempty"` // Optional message
	Data    map[string]interface{} `json:"data,omitempty"`    // Command-specific data
	Error   *JSONError             `json:"error,omitempty"`   // Error details
}

// JSONError represents error information in JSON output
type JSONError struct {
	Title   string `json:"title"`   // Error title
	Message string `json:"message"` // Error message
}
