package vcd_client

import (
	"github.com/f41gh7/vcd-csi/conf"
	"github.com/sirupsen/logrus"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
	"reflect"
	"testing"
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
		capacityBytes int64
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
				capacityBytes: 19000000,
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
				vm: &govcd.VM{VM: &types.VM{
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
				vm: &govcd.VM{VM: &types.VM{
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
			if got > 15 || got < 0 {
				t.Errorf("getFreeUnitNum() got = %v, want more than 0 and less then 16", got)
			}
			t.Logf("got: %v", got)
		})
	}
}
