package errs

// ValidationError — a bad or missing CLI argument. Param names the offending flag.
type ValidationError struct {
	Problem
	Param string `json:"param,omitempty"`
}

// AuthError — LoginBySign failed, or a session could not be recovered.
type AuthError struct {
	Problem
}

// ConfigError — missing profile or incomplete credentials. Field names the gap.
type ConfigError struct {
	Problem
	Field string `json:"field,omitempty"`
}

// NetworkError — transport failure. CauseKind is one of timeout|dns|tls|conn|5xx.
type NetworkError struct {
	Problem
	CauseKind string `json:"cause,omitempty"`
}

// APIError — a K3 business-level error. Detail preserves the raw K3 error body.
type APIError struct {
	Problem
	Detail any `json:"detail,omitempty"`
}

// ConfirmationRequiredError — reserved for future write operations that need
// --yes. Risk is read|write|high-risk-write; Action identifies the command.
type ConfirmationRequiredError struct {
	Problem
	Risk   string `json:"risk"`
	Action string `json:"action"`
}

// NewValidation builds a *ValidationError.
func NewValidation(msg, hint, param string) *ValidationError {
	return &ValidationError{Problem: Problem{Category: CategoryValidation, Message: msg, Hint: hint}, Param: param}
}

// NewAuth builds a *AuthError. code is an optional K3 message code (0 = none).
func NewAuth(msg, hint string, code int) *AuthError {
	return &AuthError{Problem: Problem{Category: CategoryAuth, Message: msg, Hint: hint, Code: code}}
}

// NewConfig builds a *ConfigError.
func NewConfig(msg, hint, field string) *ConfigError {
	return &ConfigError{Problem: Problem{Category: CategoryConfig, Message: msg, Hint: hint}, Field: field}
}

// NewNetwork builds a retryable *NetworkError.
func NewNetwork(msg, hint, causeKind string) *NetworkError {
	return &NetworkError{Problem: Problem{Category: CategoryNetwork, Message: msg, Hint: hint, Retryable: true}, CauseKind: causeKind}
}

// NewAPI builds a *APIError carrying the raw K3 error body.
func NewAPI(msg, hint string, code int, detail any) *APIError {
	return &APIError{Problem: Problem{Category: CategoryAPI, Message: msg, Hint: hint, Code: code}, Detail: detail}
}
