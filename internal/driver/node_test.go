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

package driver

import (
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/f41gh7/vcd-csi/conf"
	mock_mount "github.com/f41gh7/vcd-csi/mock/mounter"
	mock_vcd_client "github.com/f41gh7/vcd-csi/mock/vcdclient"
	"github.com/golang/mock/gomock"
	"github.com/sirupsen/logrus"
	"sync"
	"testing"
)

var (
	testConf *conf.ControllerConfig
	testLog  *logrus.Entry
)

func testInit() {
	if testConf == nil {
		cfg := &conf.ControllerConfig{}
		log := cfg.GetLogger()
		testConf = cfg
		testLog = log
	}
}

func TestCsiDriver_nodePublishVolumeForBlock(t *testing.T) {
	testInit()
	type fields struct {
		l       *logrus.Entry
		c       *conf.ControllerConfig
		wg      *sync.WaitGroup
		Name    string
		Version string
		nodeId  string
	}
	type args struct {
		req          *csi.NodePublishVolumeRequest
		mountOptions []string
		log          *logrus.Entry
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		{
			name: "test node publish",
			args: args{
				req:          &csi.NodePublishVolumeRequest{VolumeId: "some-1", PublishContext: map[string]string{pubContextUnit: "0", pubContextDiskSize: "10000000"}},
				mountOptions: nil,
				log:          testLog,
			},
			fields: fields{
				l:       testLog,
				c:       testConf,
				wg:      &sync.WaitGroup{},
				Name:    "",
				Version: "",
				nodeId:  "",
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			controller := gomock.NewController(t)

			mnt := mock_mount.NewMockMounter(controller)
			vcl := mock_vcd_client.NewMockVcdService(controller)
			mnt.EXPECT().GetDiskUnit(tt.args.req.PublishContext[pubContextDiskSize], tt.args.req.PublishContext[pubContextUnit], "").Return("/dev/sda", nil)
			mnt.EXPECT().IsMounted("").Return(true, nil)
			c := &CsiDriver{
				l:       tt.fields.l,
				c:       tt.fields.c,
				wg:      tt.fields.wg,
				vcl:     vcl,
				Name:    tt.fields.Name,
				Version: tt.fields.Version,
				nodeId:  tt.fields.nodeId,
				m:       mnt,
			}
			if err := c.nodePublishVolumeForBlock(tt.args.req, tt.args.mountOptions, tt.args.log); (err != nil) != tt.wantErr {
				t.Errorf("nodePublishVolumeForBlock() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
