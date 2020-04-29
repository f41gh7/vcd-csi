package mount

import (
	"testing"
)

func Test_extractUnitNumForScsi(t *testing.T) {
	type args struct {
		bus string
	}
	tests := []struct {
		name string
		args args
		want string
	}{
		{
			name: "splitt scsi",
			args: args{bus: "pci-0000:1b:00.0-scsi-0:0:5:0"},
			want: "5",
		},
		{
			name: "split scsi",
			args: args{bus: "pci-0000:2b:00.0-scsi-0:0:11:0"},
			want: "11",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := extractUnitNumForScsi(tt.args.bus); got != tt.want {
				t.Errorf("extractUnitNumForScsi() = %v, want %v", got, tt.want)
			}
		})
	}
}
