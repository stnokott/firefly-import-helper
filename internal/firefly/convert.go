package firefly

import (
	"github.com/stnokott/firefly-import-helper/internal/domain"
	"github.com/stnokott/firefly-import-helper/internal/firefly/generated"
)

//go:generate go tool goverter gen -output-constraint= ./

//goverter:converter
//goverter:output:format function
//goverter:output:file convert.gen.go
//goverter:output:package github.com/stnokott/firefly-import-helper/internal/firefly
type Converter interface {
	ConvertAccounts([]generated.AccountRead) []domain.FireflyAccount
	//goverter:map Id ID
	//goverter:map Attributes.Active Active | boolOrTrue
	//goverter:map Attributes.Name Name
	ConvertAccount(generated.AccountRead) domain.FireflyAccount
}

func boolOrTrue(b *bool) bool {
	if b != nil {
		return *b
	}
	return true
}
