// Package errs is the typed error contract: every error an openerp command
// returns carries a Category that the output layer maps to a stable JSON shape
// and shell exit code. Modeled on larksuite/cli's errs package (slimmed for the
// POC). The guiding rule (from that CLI's AGENTS.md): every error message is
// parsed by an AI agent to decide its next action, so errors must be
// structured, actionable, and carry a hint.
package errs

import "errors"

// Category is the coarse error class. It drives both the wire "type" field and
// the process exit code (see internal/output/exitcode.go).
type Category string

const (
	CategoryValidation   Category = "validation"     // bad/missing argument
	CategoryAuth         Category = "authentication" // LoginBySign failed / session unrecoverable
	CategoryConfig       Category = "configuration"  // missing profile / credentials
	CategoryNetwork      Category = "network"        // timeout / DNS / connection reset
	CategoryAPI          Category = "api"            // K3 business-level error
	CategoryInternal     Category = "internal"       // should-not-happen
	CategoryConfirmation Category = "confirmation"   // reserved: write op needs --yes
)

// ProblemDetailer is implemented by every typed error via its embedded Problem.
type ProblemDetailer interface {
	ProblemDetail() *Problem
}

// ProblemOf extracts the embedded *Problem from anywhere in the error chain.
func ProblemOf(err error) (*Problem, bool) {
	var pd ProblemDetailer
	if errors.As(err, &pd) {
		if p := pd.ProblemDetail(); p != nil {
			return p, true
		}
	}
	return nil, false
}

// CategoryOf returns the error's Category, defaulting to CategoryInternal for
// untyped errors.
func CategoryOf(err error) Category {
	if p, ok := ProblemOf(err); ok {
		return p.Category
	}
	return CategoryInternal
}
