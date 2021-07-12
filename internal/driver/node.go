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
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func (c *CsiDriver) NodeStageVolume(_ context.Context, req *csi.NodeStageVolumeRequest) (*csi.NodeStageVolumeResponse, error) {
	l := c.l.WithFields(logrus.Fields{"volume": req.VolumeId, "context": req.PublishContext})
	l.Infof("get request for stage")
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Volume ID must be provided")
	}

	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Staging Target Path must be provided")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "NodeStageVolume Volume Capability must be provided")
	}
	var diskUnit, diskSize string

	if value, ok := req.PublishContext[pubContextUnit]; ok {
		diskUnit = value
	} else {
		l.Errorf("cannot find disk unit at context")
		return nil, status.Error(codes.InvalidArgument, "Publish context diskUnitNum must be provided")

	}
	if value, ok := req.PublishContext[pubContextDiskSize]; ok {
		diskSize = value
	} else {
		l.Errorf("cannot find disk unit at context")
		return nil, status.Error(codes.InvalidArgument, "Publish context diskSizeB must be provided")

	}

	// If it is a block volume, we do nothing for stage volume
	// because we bind mount the absolute device path to a file
	switch req.VolumeCapability.GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
		return &csi.NodeStageVolumeResponse{}, nil
	}
	//so, the most interesting part
	//we need to guess, where is our disk
	//what options we have:
	// size in bytes
	//unit number

	source, err := c.m.GetDiskUnit(diskSize, diskUnit, "")
	if err != nil {
		l.WithError(err).Errorf("cannot get source ")
		return nil, err
	}
	target := req.StagingTargetPath

	mnt := req.VolumeCapability.GetMount()
	options := mnt.MountFlags

	fsType := "ext4"
	if mnt.FsType != "" {
		fsType = mnt.FsType
	}

	formatted, err := c.m.IsFormatted(source)
	if err != nil {
		return nil, err
	}

	if !formatted {
		l.Info("formatting the volume for staging")
		if err := c.m.Format(source, fsType); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		l.Info("source device is already formatted")
	}

	l.Info("mounting the volume for staging")

	mounted, err := c.m.IsMounted(target)
	if err != nil {
		return nil, err
	}

	if !mounted {
		if err := c.m.Mount(source, target, fsType, options...); err != nil {
			return nil, status.Error(codes.Internal, err.Error())
		}
	} else {
		l.Info("source device is already mounted to the target path")
	}

	l.Info("formatting and mounting stage volume is finished")
	return &csi.NodeStageVolumeResponse{}, nil
}

func (c *CsiDriver) NodeUnstageVolume(_ context.Context, req *csi.NodeUnstageVolumeRequest) (*csi.NodeUnstageVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeUnstageVolume Volume ID must be provided")
	}

	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeUnstageVolume Staging Target Path must be provided")
	}

	l := c.l.WithFields(logrus.Fields{
		"volume_id":           req.VolumeId,
		"staging_target_path": req.StagingTargetPath,
	})
	l.Info("node unstage volume called")

	mounted, err := c.m.IsMounted(req.StagingTargetPath)
	if err != nil {
		return nil, err
	}

	if mounted {
		l.Info("unmounting the staging target path")
		err := c.m.Unmount(req.StagingTargetPath)
		if err != nil {
			return nil, err
		}
	} else {
		l.Info("staging target path is already unmounted")
	}

	l.Info("unmounting stage volume is finished")
	return &csi.NodeUnstageVolumeResponse{}, nil
}

func (c *CsiDriver) NodePublishVolume(_ context.Context, req *csi.NodePublishVolumeRequest) (*csi.NodePublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume Volume ID must be provided")
	}

	if req.StagingTargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume Staging Target Path must be provided")
	}

	if req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume Target Path must be provided")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "NodePublishVolume Volume Capability must be provided")
	}

	l := c.l.WithFields(logrus.Fields{
		"volume_id":           req.VolumeId,
		"staging_target_path": req.StagingTargetPath,
		"target_path":         req.TargetPath,
	})
	l.Info("node publish volume called")

	options := []string{"bind"}
	if req.Readonly {
		options = append(options, "ro")
	}

	var err error
	switch req.GetVolumeCapability().GetAccessType().(type) {
	case *csi.VolumeCapability_Block:
		err = c.nodePublishVolumeForBlock(req, options, l)
	case *csi.VolumeCapability_Mount:
		err = c.nodePublishVolumeForFileSystem(req, options, l)
	default:
		return nil, status.Error(codes.InvalidArgument, "Unknown access type")
	}

	if err != nil {
		return nil, err
	}

	l.Info("bind mounting the volume is finished")
	return &csi.NodePublishVolumeResponse{}, nil
}

func (c *CsiDriver) NodeUnpublishVolume(_ context.Context, req *csi.NodeUnpublishVolumeRequest) (*csi.NodeUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolume Volume ID must be provided")
	}

	if req.TargetPath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeUnpublishVolume Target Path must be provided")
	}

	l := c.l.WithFields(logrus.Fields{
		"volume_id":   req.VolumeId,
		"target_path": req.TargetPath,
	})
	l.Info("node unpublish volume called")

	mounted, err := c.m.IsMounted(req.TargetPath)
	if err != nil {
		return nil, err
	}

	if mounted {
		l.Info("unmounting the target path")
		err := c.m.Unmount(req.TargetPath)
		if err != nil {
			return nil, err
		}
	} else {
		l.Info("target path is already unmounted")
	}

	l.Info("unmounting volume is finished")
	return &csi.NodeUnpublishVolumeResponse{}, nil
}

func (c *CsiDriver) NodeGetVolumeStats(_ context.Context, req *csi.NodeGetVolumeStatsRequest) (*csi.NodeGetVolumeStatsResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats Volume ID must be provided")
	}

	volumePath := req.VolumePath
	if volumePath == "" {
		return nil, status.Error(codes.InvalidArgument, "NodeGetVolumeStats Volume Path must be provided")
	}

	l := c.l.WithFields(logrus.Fields{
		"volume_id":   req.VolumeId,
		"volume_path": req.VolumePath,
	})
	l.Info("node get volume stats called")

	mounted, err := c.m.IsMounted(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to check if volume path %q is mounted: %s", volumePath, err)
	}

	if !mounted {
		return nil, status.Errorf(codes.NotFound, "volume path %q is not mounted", volumePath)
	}

	isBlock, err := c.m.IsBlockDevice(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to determine if %q is block device: %s", volumePath, err)
	}

	stats, err := c.m.GetStatistics(volumePath)
	if err != nil {
		return nil, status.Errorf(codes.Internal, "failed to retrieve capacity statistics for volume path %q: %s", volumePath, err)
	}

	// only can retrieve total capacity for a block device
	if isBlock {
		l.WithFields(logrus.Fields{
			"volume_mode": volumeModeBlock,
			"bytes_total": stats.TotalBytes,
		}).Info("node capacity statistics retrieved")

		return &csi.NodeGetVolumeStatsResponse{
			Usage: []*csi.VolumeUsage{
				{
					Unit:  csi.VolumeUsage_BYTES,
					Total: stats.TotalBytes,
				},
			},
		}, nil
	}

	l.WithFields(logrus.Fields{
		"volume_mode":      volumeModeFilesystem,
		"bytes_available":  stats.AvailableBytes,
		"bytes_total":      stats.TotalBytes,
		"bytes_used":       stats.UsedBytes,
		"inodes_available": stats.AvailableInodes,
		"inodes_total":     stats.TotalInodes,
		"inodes_used":      stats.UsedInodes,
	}).Info("node capacity statistics retrieved")

	return &csi.NodeGetVolumeStatsResponse{
		Usage: []*csi.VolumeUsage{
			{
				Available: stats.AvailableBytes,
				Total:     stats.TotalBytes,
				Used:      stats.UsedBytes,
				Unit:      csi.VolumeUsage_BYTES,
			},
			{
				Available: stats.AvailableInodes,
				Total:     stats.TotalInodes,
				Used:      stats.UsedInodes,
				Unit:      csi.VolumeUsage_INODES,
			},
		},
	}, nil
}

func (c *CsiDriver) NodeExpandVolume(_ context.Context, req *csi.NodeExpandVolumeRequest) (*csi.NodeExpandVolumeResponse, error) {
	l := c.l.WithFields(logrus.Fields{"volume": req.VolumeId})

	l.Infof("volume expand req")
	return nil, nil
}

func (c *CsiDriver) NodeGetCapabilities(context.Context, *csi.NodeGetCapabilitiesRequest) (*csi.NodeGetCapabilitiesResponse, error) {
	l := c.l.WithFields(logrus.Fields{})
	l.Infof("node capabilites")
	nscaps := []*csi.NodeServiceCapability{
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_STAGE_UNSTAGE_VOLUME,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_EXPAND_VOLUME,
				},
			},
		},
		{
			Type: &csi.NodeServiceCapability_Rpc{
				Rpc: &csi.NodeServiceCapability_RPC{
					Type: csi.NodeServiceCapability_RPC_GET_VOLUME_STATS,
				},
			},
		},
	}

	return &csi.NodeGetCapabilitiesResponse{
		Capabilities: nscaps,
	}, nil
}

func (c *CsiDriver) NodeGetInfo(context.Context, *csi.NodeGetInfoRequest) (*csi.NodeGetInfoResponse, error) {
	c.l.Infof("node info req")
	return &csi.NodeGetInfoResponse{
		NodeId:            c.nodeId,
		MaxVolumesPerNode: maxVolumesPerNode,

		// make sure that the driver works on this particular region only
		AccessibleTopology: &csi.Topology{
			Segments: map[string]string{
				topologyKey: c.c.NodeVdc,
			},
		},
	}, nil
}

func (c *CsiDriver) nodePublishVolumeForFileSystem(req *csi.NodePublishVolumeRequest, mountOptions []string, log *logrus.Entry) error {
	source := req.StagingTargetPath
	target := req.TargetPath

	mnt := req.VolumeCapability.GetMount()
	for _, flag := range mnt.MountFlags {
		mountOptions = append(mountOptions, flag)
	}

	fsType := "ext4"
	if mnt.FsType != "" {
		fsType = mnt.FsType
	}

	mounted, err := c.m.IsMounted(target)
	if err != nil {
		return err
	}

	log = log.WithFields(logrus.Fields{
		"source_path":   source,
		"volume_mode":   volumeModeFilesystem,
		"fs_type":       fsType,
		"mount_options": mountOptions,
	})

	if !mounted {
		log.Info("mounting the volume")
		if err := c.m.Mount(source, target, fsType, mountOptions...); err != nil {
			return status.Error(codes.Internal, err.Error())
		}
	} else {
		log.Info("volume is already mounted")
	}

	return nil
}

func (c *CsiDriver) nodePublishVolumeForBlock(req *csi.NodePublishVolumeRequest, mountOptions []string, log *logrus.Entry) error {
	var diskUnit, diskSize string

	if value, ok := req.PublishContext[pubContextUnit]; ok {
		diskUnit = value
	} else {
		c.l.Errorf("cannot find disk unit at context")
		return status.Error(codes.InvalidArgument, "Publish context diskUnitNum must be provided")

	}
	if value, ok := req.PublishContext[pubContextDiskSize]; ok {
		diskSize = value
	} else {
		c.l.Errorf("cannot find disk unit at context")
		return status.Error(codes.InvalidArgument, "Publish context diskSizeB must be provided")

	}
	source, err := c.m.GetDiskUnit(diskSize, diskUnit, "")
	if err != nil {
		return status.Errorf(codes.Internal, "Failed to find device path for volume %s. %v", req.VolumeId, err)
	}

	target := req.TargetPath

	mounted, err := c.m.IsMounted(target)
	if err != nil {
		return err
	}

	log = log.WithFields(logrus.Fields{
		"source_path":   source,
		"volume_mode":   volumeModeBlock,
		"mount_options": mountOptions,
	})

	if !mounted {
		log.Info("mounting the volume")
		if err := c.m.Mount(source, target, "", mountOptions...); err != nil {
			return status.Errorf(codes.Internal, err.Error())
		}
	} else {
		log.Info("volume is already mounted")
	}

	return nil
}
