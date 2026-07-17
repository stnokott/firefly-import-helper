package telegram

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDrawHTMLTable(t *testing.T) {
	t.Parallel()

	headers := []string{"ColA", "ColBB", "ColCCC"}
	rows := [][]string{
		{"foo", "bar", "foobar"},
		{"fuzz", "buzz", "long value here"},
		{"long value here", "a", "b"},
	}

	want := `<pre>+-----------------+-------+-----------------+
| ColA            | ColBB | ColCCC          |
+-----------------+-------+-----------------+
| foo             | bar   | foobar          |
| fuzz            | buzz  | long value here |
| long value here | a     | b               |
+-----------------+-------+-----------------+
</pre>
`
	got := drawHTMLTable(headers, rows)
	assert.Equal(t, want, got)
}
