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
	"context"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/f41gh7/vcd-csi/conf"
	"github.com/f41gh7/vcd-csi/internal/locker"
	"github.com/f41gh7/vcd-csi/internal/mount"
	vcd_client "github.com/f41gh7/vcd-csi/pkg/vcd-client"
	"github.com/golang/protobuf/ptypes/wrappers"
	"github.com/sirupsen/logrus"
	"sync"
)

const (
	maxVolumesPerNode    = 8
	volumeModeBlock      = "block"
	volumeModeFilesystem = "filesystem"
)

type CsiDriver struct {
	l       *logrus.Entry
	lock locker.Locker
	c       *conf.ControllerConfig
	wg      *sync.WaitGroup
	vcl     vcd_client.VcdService
	Name    string
	Version string
	nodeId  string
	m       mount.Mounter
}

func NewCsiDriver(l *logrus.Entry, c *conf.ControllerConfig, VcdClient vcd_client.VcdService, m mount.Mounter,lock locker.Locker,  wg *sync.WaitGroup) (*CsiDriver, error) {

	cd := &CsiDriver{
		l:      l,
		c:      c,
		wg:     wg,
		vcl:    VcdClient,
		Name:   c.ControllerName,
		nodeId: c.NodeName,
		lock:lock,
		m:      m,
	}

	return cd, nil
}

func (c *CsiDriver) GetPluginInfo(context.Context, *csi.GetPluginInfoRequest) (*csi.GetPluginInfoResponse, error) {
	resp := &csi.GetPluginInfoResponse{
		Name:          c.Name,
		VendorVersion: c.Version,
	}

	c.l.WithFields(logrus.Fields{
		"response": resp,
		"method":   "get_plugin_info",
	}).Info("get plugin info called")
	return resp, nil
}

func (c *CsiDriver) GetPluginCapabilities(context.Context, *csi.GetPluginCapabilitiesRequest) (*csi.GetPluginCapabilitiesResponse, error) {
	resp := &csi.GetPluginCapabilitiesResponse{
		Capabilities: []*csi.PluginCapability{
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_CONTROLLER_SERVICE,
					},
				},
			},
			{
				Type: &csi.PluginCapability_Service_{
					Service: &csi.PluginCapability_Service{
						Type: csi.PluginCapability_Service_VOLUME_ACCESSIBILITY_CONSTRAINTS,
					},
				},
			},
			{
				Type: &csi.PluginCapability_VolumeExpansion_{
					VolumeExpansion: &csi.PluginCapability_VolumeExpansion{
						Type: csi.PluginCapability_VolumeExpansion_OFFLINE,
					},
				},
			},
		},
	}

	c.l.WithFields(logrus.Fields{
		"response": resp,
		"method":   "get_plugin_capabilities",
	}).Info("get plugin capabitilies called")
	return resp, nil
}

func (c *CsiDriver) Probe(context.Context, *csi.ProbeRequest) (*csi.ProbeResponse, error) {
	c.l.WithField("method", "probe").Info("probe called")
	//TODO check

	return &csi.ProbeResponse{
		Ready: &wrappers.BoolValue{
			Value: true,
		},
	}, nil
}

//
