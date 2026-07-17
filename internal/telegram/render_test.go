package telegram

import (
	"testing"
	"time"

	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRenderNewTransaction(t *testing.T) {
	t.Parallel()

	data := &domain.TransactionCreated{
		Type:           domain.TransactionTypeWithdrawal,
		AccountName:    "My Account A",
		MerchantName:   "Pirates",
		Amount:         12.34567,
		CurrencySymbol: "€",
		Date:           time.Date(2026, 1, 1, 1, 1, 1, 1, time.UTC),
		Description:    "Gold tax!",
		FireflyID:      "5566",
		FireflyURL:     "https://my.firefly.instance.com/transactions/5566",
	}

	want := `✨ <b>New Transaction <a href="https://my.firefly.instance.com/transactions/5566">#5566</a></b> ✨

<b>My Account A →12.35€→ Pirates</b>

<b>Description:</b> Gold tax!
<b>Occurred:</b> <tg-time unix="1767229261" format="r">2026-01-01 01:01:01</tg-time>
`

	got, err := renderNewTransaction(data)
	require.NoError(t, err)
	assert.Equal(t, want, got)
}

func TestRenderHTMLTable(t *testing.T) {
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
	got := renderHTMLTable(headers, rows)
	assert.Equal(t, want, got)
}
