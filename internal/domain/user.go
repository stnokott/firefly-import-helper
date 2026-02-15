package domain

import (
	"regexp"
	"strings"

	"github.com/oapi-codegen/runtime/types"
)

func CensorEmail(e types.Email) string {
	rx := regexp.MustCompile(`^(\S+)@([^\s\.]+)\.(\S+)$`)
	matches := rx.FindStringSubmatch(string(e))
	if matches == nil || len(matches) != 4 {
		logger.Warn("attempted to censor invalid email")
		return ""
	}
	var b strings.Builder
	b.Grow(len(e))
	b.WriteString(censorString(matches[1], 3))
	b.WriteRune('@')
	b.WriteString(censorString(matches[2], 0))
	b.WriteRune('.')
	b.WriteString(matches[3])
	return b.String()
}

// censorString returns s, with the first maxClear characters retained and the remaining characters
// replaced with an asterisk.
//
// Example: censorString("123foo", 2) // returns "12****"
func censorString(s string, maxClear int) string {
	if len(s) <= maxClear {
		return strings.Repeat("*", len(s))
	}
	return s[:maxClear] + strings.Repeat("*", len(s)-maxClear)
}
