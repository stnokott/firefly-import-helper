package domain

import (
	"testing"

	"github.com/oapi-codegen/runtime/types"
	"github.com/stretchr/testify/assert"
)

func TestCensorEmail(t *testing.T) {
	tests := []struct {
		title string
		email types.Email
		want  string
	}{
		{
			title: "",
			email: "foobar123@gmail.com",
			want:  "foo******@*****.com",
		},
		{
			title: "short",
			email: "a@b.c",
			want:  "*@*.c",
		},
		{
			title: "double domain ending",
			email: "foobar@email.co.uk",
			want:  "foo***@*****.co.uk",
		},
		{
			title: "subdomain",
			email: "foobar@email.domain.com",
			want:  "foo***@*****.domain.com",
		},
	}
	for _, tt := range tests {
		t.Run(tt.title, func(t *testing.T) {
			got := CensorEmail(tt.email)
			assert.Equal(t, tt.want, got)
		})
	}
}
