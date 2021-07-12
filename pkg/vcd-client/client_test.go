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

package vcd_client

import (
	"reflect"
	"sync"
	"testing"

	"github.com/f41gh7/vcd-csi/conf"
	types2 "github.com/f41gh7/vcd-csi/pkg/types"
	"github.com/sirupsen/logrus"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
)

var (
	testConf *conf.ControllerConfig
	testLog  *logrus.Entry
)

/*
this tests doesnt work atm
need to decide how to mock it/int test correctly
*/

func testInit() {
	if testConf == nil {
		cfg, err := conf.NewControllerConfig()
		if err != nil {
			panic(err)
		}
		testConf = cfg
		testLog = testConf.GetLogger()
	}
}
func TestVcdClient_ListVolumes(t *testing.T) {
	testInit()
	type fields struct {
		c          *conf.ControllerConfig
		l          *logrus.Entry
		Client     *govcd.VCDClient
		Org        *govcd.Org
		VdcClients map[string]*govcd.Vdc
	}
	tests := []struct {
		name    string
		fields  fields
		want    []*VolumeWithCap
		wantErr bool
	}{
		{
			name: "list volume",
			fields: fields{
				c: testConf,
				l: testLog,
			},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVcdClient(tt.fields.c, tt.fields.l)
			if err != nil {
				t.Errorf("error setuping client for test: %v", err)
			}
			got, err := v.ListVolumes()
			if (err != nil) != tt.wantErr {
				t.Errorf("ListVolumes() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListVolumes() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVcdClient_CreateDisk(t *testing.T) {
	testInit()
	type fields struct {
		c          *conf.ControllerConfig
		l          *logrus.Entry
		Client     *govcd.VCDClient
		Org        *govcd.Org
		VdcClients map[string]*govcd.Vdc
	}
	type args struct {
		vcd           string
		name          string
		profile       string
		capacityBytes *types2.StorageSize
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "Create simple disk",
			args: args{
				vcd:           testConf.Vdcs[0],
				name:          "test-pvc-1",
				profile:       "FAS-Basic",
				capacityBytes: types2.NewStorageSize( 19000000),
			},
			fields: fields{
				c: testConf,
				l: testLog,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVcdClient(tt.fields.c, tt.fields.l)
			if err != nil {
				t.Errorf("cannot setup tests: %v", err)
				return
			}
			if err := v.CreateDisk(tt.args.vcd, tt.args.name, tt.args.profile, tt.args.capacityBytes); (err != nil) != tt.wantErr {
				t.Errorf("CreateDisk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVcdClient_DeleteDisk(t *testing.T) {
	testInit()
	type fields struct {
		c          *conf.ControllerConfig
		l          *logrus.Entry
		Client     *govcd.VCDClient
		Org        *govcd.Org
		VdcClients map[string]*govcd.Vdc
	}
	type args struct {
		vcd  string
		name string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "delete volume",
			args: args{
				name: "test-pvc-1",
			},
			fields: fields{
				c: testConf,
				l: testLog,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVcdClient(tt.fields.c, tt.fields.l)
			if err != nil {
				t.Errorf("cannot setup tests: %v", err)
				return
			}

			if err := v.DeleteDisk(tt.args.name); (err != nil) != tt.wantErr {
				t.Errorf("DeleteDisk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestVcdClient_getVmByName(t *testing.T) {
	testInit()
	type fields struct {
		c          *conf.ControllerConfig
		l          *logrus.Entry
		Client     *govcd.VCDClient
		Org        *govcd.Org
		VdcClients map[string]*govcd.Vdc
	}
	type args struct {
		client *govcd.Vdc
		vm     string
	}
	tests := []struct {
		name   string
		fields fields
		args   args
	}{
		{
			name: "list vapps",
			fields: fields{
				c: testConf,
				l: testLog,
			},
			args: args{
				client: nil,
				vm:     "172.30.37.29",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVcdClient(tt.fields.c, tt.fields.l)
			if err != nil {
				t.Errorf("failed to setup test: %v", err)
				return
			}
			for _, value := range v.VdcClients {
				v.getVMByName(value, tt.args.vm)
			}
		})
	}
}

func TestVcdClient_AttachDisk(t *testing.T) {
	testInit()
	type fields struct {
		c          *conf.ControllerConfig
		l          *logrus.Entry
		Client     *govcd.VCDClient
		Org        *govcd.Org
		VdcClients map[string]*govcd.Vdc
	}
	type args struct {
		vm   string
		disk string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *DiskAttachParams
		wantErr bool
	}{
		{
			name: "attach to test vm disk",
			args: args{
				vm:   "t-2",
				disk: "tp3",
			},
			fields: fields{
				c: testConf,
				l: testLog,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVcdClient(tt.fields.c, tt.fields.l)
			if err != nil {
				t.Errorf("cannot setup tests: %v", err)
				return
			}
			got, err := v.AttachDisk(tt.args.vm, tt.args.disk)
			if (err != nil) != tt.wantErr {
				t.Errorf("AttachDisk() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AttachDisk() got = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestVcdClient_DettachDisk(t *testing.T) {
	testInit()
	type fields struct {
		c          *conf.ControllerConfig
		l          *logrus.Entry
		Client     *govcd.VCDClient
		Org        *govcd.Org
		VdcClients map[string]*govcd.Vdc
	}
	type args struct {
		vm   string
		disk string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "detach disk",
			args: args{
				vm:   "t-2",
				disk: "tp1",
			},
			fields: fields{
				c: testConf,
				l: testLog,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVcdClient(tt.fields.c, tt.fields.l)
			if err != nil {
				t.Errorf("cannot setup test :%v", err)
				return
			}
			if err := v.DetachDisk(tt.args.vm, tt.args.disk); (err != nil) != tt.wantErr {
				t.Errorf("DetachDisk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_getFreeUnitNum(t *testing.T) {
	type args struct {
		vm     *govcd.VM
		busNum int
	}
	tests := []struct {
		name    string
		args    args
		want    int
		wantErr bool
	}{
		{
			name: "excract some num",
			args: args{
				vm: &govcd.VM{VM: &types.Vm{
					VmSpecSection: &types.VmSpecSection{
						DiskSection: &types.DiskSection{DiskSettings: []*types.DiskSettings{
							&types.DiskSettings{UnitNumber: 2, BusNumber: 3},
							&types.DiskSettings{UnitNumber: 0, BusNumber: 0},
							&types.DiskSettings{UnitNumber: 2, BusNumber: 3},
							&types.DiskSettings{UnitNumber: 3, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 3, BusNumber: 6},
						},
						},
					},
				}},
				busNum: 2,
			},
			wantErr: false,
		},
		{
			name: "no free units some num",
			args: args{
				vm: &govcd.VM{VM: &types.Vm{
					VmSpecSection: &types.VmSpecSection{
						DiskSection: &types.DiskSection{DiskSettings: []*types.DiskSettings{
							&types.DiskSettings{UnitNumber: 0, BusNumber: 0},
							&types.DiskSettings{UnitNumber: 3, BusNumber: 1},
							&types.DiskSettings{UnitNumber: 2, BusNumber: 0},
							&types.DiskSettings{UnitNumber: 2, BusNumber: 1},
							&types.DiskSettings{UnitNumber: 2, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 2, BusNumber: 3},

							&types.DiskSettings{UnitNumber: 0, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 1, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 2, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 3, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 4, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 5, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 6, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 8, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 9, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 10, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 11, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 12, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 13, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 14, BusNumber: 2},
							&types.DiskSettings{UnitNumber: 15, BusNumber: 2},
						},
						},
					},
				}},
				busNum: 2,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := getFreeUnitNum(tt.args.vm, tt.args.busNum)
			if (err != nil) != tt.wantErr {
				t.Errorf("getFreeUnitNum() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got > 15 || got < 8 {
				t.Errorf("getFreeUnitNum() got = %v, want more than 0 and less then 16", got)
			}
			t.Logf("got: %v", got)
		})
	}
}

func TestVcdClient_ResizeDisk(t *testing.T) {
	testInit()
	type fields struct {
		c          *conf.ControllerConfig
		l          *logrus.Entry
		VdcClients map[string]*govcd.Vdc
		mt         *sync.Mutex
		baseClient *govcd.VCDClient
	}
	type args struct {
		vdcName     string
		diskName    string
		newDiskSize *types2.StorageSize
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name:"resize test disk",
			args:args{
				vdcName:     "",
				diskName:    "tp3",
				newDiskSize: types2.NewStorageSize(17000928256),
			},
			fields:fields{
				c:          testConf,
				l:          testLog,
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v, err := NewVcdClient(tt.fields.c, tt.fields.l)
			if err != nil {
				t.Errorf("cannot setup test :%v", err)
				return
			}

			if err := v.ResizeDisk(tt.args.vdcName, tt.args.diskName, tt.args.newDiskSize); (err != nil) != tt.wantErr {
				t.Errorf("ResizeDisk() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}