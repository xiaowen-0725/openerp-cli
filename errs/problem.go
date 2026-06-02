package errs

// Problem is the shared shape embedded by every typed error. Its fields are
// promoted to the top level of the JSON error envelope (the embedding structs
// add their own extension fields alongside). Message is REQUIRED.
type Problem struct {
	Category  Category `json:"type"`
	Subtype   string   `json:"subtype,omitempty"`
	Code      int      `json:"code,omitempty"`
	Message   string   `json:"message"`
	Hint      string   `json:"hint,omitempty"`
	Retryable bool     `json:"retryable,omitempty"`
}

// Error satisfies the error interface. A nil receiver yields "" so a stray nil
// *Problem in an error interface cannot panic callers.
func (p *Problem) Error() string {
	if p == nil {
		return ""
	}
	return p.Message
}

// ProblemDetail exposes the embedded Problem to ProblemOf/CategoryOf.
func (p *Problem) ProblemDetail() *Problem { return p }
