package driver

import (
	"context"
	"github.com/container-storage-interface/spec/lib/go/csi"
	"github.com/f41gh7/vcd-csi/conf"
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
	c       *conf.ControllerConfig
	wg      *sync.WaitGroup
	vcl     vcd_client.VcdService
	Name    string
	Version string
	nodeId  string
	m       mount.Mounter
}

func NewCsiDriver(l *logrus.Entry, c *conf.ControllerConfig, VcdClient vcd_client.VcdService, m mount.Mounter, wg *sync.WaitGroup) (*CsiDriver, error) {

	cd := &CsiDriver{
		l:      l,
		c:      c,
		wg:     wg,
		vcl:    VcdClient,
		Name:   c.ControllerName,
		nodeId: c.NodeName,
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
