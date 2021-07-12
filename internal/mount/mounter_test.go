/*
 * Copyright (c) 2020   f41gh7
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 */

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
