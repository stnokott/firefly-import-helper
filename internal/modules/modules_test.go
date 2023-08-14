package modules

import (
	"firefly-iii-fix-ing/internal/structs"
	"reflect"
	"testing"
)

func TestModuleIngDescriptionFormatProcess(t *testing.T) {
	type args struct {
		s *structs.TransactionSplitUpdate
	}
	tests := []struct {
		name    string
		args    args
		want    *structs.TransactionSplitUpdate
		wantErr bool
	}{
		{
			"all fields filled",
			args{s: &structs.TransactionSplitUpdate{Description: "mandatereference:mRef,creditorid:credId,remittanceinformation:RemInf"}},
			&structs.TransactionSplitUpdate{Description: "RemInf", CreditorId: "credId", MandateReference: "mRef"},
			false,
		},
		{
			"one field empty",
			args{s: &structs.TransactionSplitUpdate{Description: "mandatereference:mRef,creditorid:,remittanceinformation:RemInf"}},
			&structs.TransactionSplitUpdate{Description: "RemInf", MandateReference: "mRef"},
			false,
		},
		{
			"description placeholder",
			args{s: &structs.TransactionSplitUpdate{Description: "mandatereference:mRef,creditorid:credId,remittanceinformation:"}},
			&structs.TransactionSplitUpdate{Description: "n/a", CreditorId: "credId", MandateReference: "mRef"},
			false,
		},
		{
			"all fields empty",
			args{s: &structs.TransactionSplitUpdate{Description: "mandatereference:,creditorid:,remittanceinformation:"}},
			&structs.TransactionSplitUpdate{Description: "n/a"},
			false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := &moduleIngDescriptionFormat{}
			got, err := m.process(tt.args.s)
			if (err != nil) != tt.wantErr {
				t.Errorf("process() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("process() got = %v, want %v", got, tt.want)
			}
		})
	}
}
