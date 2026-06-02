package output

import "github.com/zhoujw/openerp-cli/errs"

// Shell exit codes. Fine-grained classes are carried in the JSON envelope's
// "type" field, not the exit code (many categories map to one code).
const (
	ExitOK                   = 0  // success
	ExitAPI                  = 1  // K3 business / generic API error
	ExitValidation           = 2  // bad argument
	ExitAuth                 = 3  // auth or config failure
	ExitNetwork              = 4  // transport failure
	ExitInternal             = 5  // should-not-happen
	ExitConfirmationRequired = 10 // reserved: write op needs --yes
)

// ExitCodeForCategory maps an errs.Category to a shell exit code (many-to-one).
func ExitCodeForCategory(cat errs.Category) int {
	switch cat {
	case errs.CategoryValidation:
		return ExitValidation
	case errs.CategoryAuth, errs.CategoryConfig:
		return ExitAuth
	case errs.CategoryNetwork:
		return ExitNetwork
	case errs.CategoryAPI:
		return ExitAPI
	case errs.CategoryConfirmation:
		return ExitConfirmationRequired
	case errs.CategoryInternal:
		return ExitInternal
	}
	return ExitInternal
}

// ExitCodeOf returns the shell exit code for any error.
func ExitCodeOf(err error) int {
	if err == nil {
		return ExitOK
	}
	if _, ok := errs.ProblemOf(err); ok {
		return ExitCodeForCategory(errs.CategoryOf(err))
	}
	return ExitInternal
}
