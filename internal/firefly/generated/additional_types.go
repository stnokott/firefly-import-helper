package generated

import (
	"regexp"
	"strconv"
	"strings"
)

type Amount float64

func (a *Amount) UnmarshalJSON(v []byte) error {
	vStr := strings.Trim(string(v), `"`)
	out, err := strconv.ParseFloat(vStr, 64)
	if err != nil {
		return err
	}
	*a = Amount(out)
	return nil
}

// Implement error interface for error responses.

func (r BadRequestResponse) Error() string {
	if r.Message == nil {
		return "bad request"
	}
	return "bad request: " + *r.Message
}

func (r UnauthenticatedResponse) Error() string {
	if r.Message == nil {
		return "unauthenticated"
	}
	return "unauthenticated: " + *r.Message
}

func (r NotFoundResponse) Error() string {
	if r.Message == nil {
		return "not found"
	}
	return "not found: " + *r.Message
}

var regexDuplicateTransactionErr = regexp.MustCompile(`(?i)^duplicate of transaction #(\d+)`)

func (r ValidationErrorResponse) Error() string {
	if r.Message == nil {
		return "validation error"
	}
	return "validation error: " + *r.Message
}

// IsDuplicateTransactionErr returns true if this error signals a duplicate transaction.
// If true, it also returns the ID of the duplicated transaction.
func (r ValidationErrorResponse) IsDuplicateTransactionErr() (string, bool) {
	if r.Message == nil {
		return "", false
	}

	if matches := regexDuplicateTransactionErr.FindStringSubmatch(*r.Message); len(matches) == 2 {
		return matches[1], true
	}
	return "", false
}

func (r InternalExceptionResponse) Error() string {
	if r.Message == nil {
		return "internal exception"
	}
	return "internal exception: " + *r.Message
}
