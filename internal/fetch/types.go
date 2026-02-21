package fetch

// Error codes returned in FetchError.Code. Callers should switch on these
// rather than parsing error message strings.
const (
	ErrCodeInvalidURL         = "invalid_url"
	ErrCodeBlockedDestination = "blocked_destination"
	ErrCodeTimeout            = "timeout"
	ErrCodeHTTPError          = "http_error"
	ErrCodeUnsupportedType    = "unsupported_content_type"
	ErrCodeNetworkError       = "network_error"
	ErrCodeConversionFailed   = "conversion_failed"
	ErrCodeInvalidRange       = "invalid_range"
)

// FetchRequest carries parameters for a single fetch operation.
type FetchRequest struct {
	URL            string
	MaxBytes       int
	StartBytes     int
	Raw            bool
	TimeoutSeconds int
}

// FetchResult is the structured return value from FetchURL.
type FetchResult struct {
	URL            string
	FinalURL       string
	StatusCode     int
	ContentType    string
	Content        string
	ContentBytes   int
	ReturnedBytes  int
	Truncated      bool
	NextStartBytes *int
	CacheHit       bool
	FetchedAt      string
}

// FetchError is a structured error with a stable code, a human-readable
// message, and optional detail fields.
type FetchError struct {
	Code    string
	Message string
	Details map[string]any
}

func (e *FetchError) Error() string { return e.Code + ": " + e.Message }

// newFetchError constructs a FetchError and satisfies the error interface.
func newFetchError(code, message string, details map[string]any) *FetchError {
	return &FetchError{Code: code, Message: message, Details: details}
}

// ErrorCode extracts the stable code from an error if it is a *FetchError,
// otherwise returns ErrCodeNetworkError as a fallback.
func ErrorCode(err error) string {
	if fe, ok := err.(*FetchError); ok {
		return fe.Code
	}
	return ErrCodeNetworkError
}
