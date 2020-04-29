package driver

import (
	"context"
	"errors"
	"fmt"
	"github.com/container-storage-interface/spec/lib/go/csi"
	vcd_client "github.com/f41gh7/vcd-csi/pkg/vcd-client"
	"github.com/sirupsen/logrus"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"k8s.io/apimachinery/pkg/util/sets"
	"strconv"
	"strings"
)

const (
	_   = iota
	kiB = 1 << (10 * iota)
	miB
	giB
	tiB
	// minimumVolumeSizeInBytes is used to validate that the user is not trying
	// to create a volume that is smaller than what we support
	minimumVolumeSizeInBytes int64 = 4194304

	// maximumVolumeSizeInBytes is used to validate that the user is not trying
	// to create a volume that is larger than what we support
	maximumVolumeSizeInBytes int64 = 4398046511104

	// defaultVolumeSizeInBytes is used when the user did not provide a size or
	// the size they provided did not satisfy our requirements
	defaultVolumeSizeInBytes int64 = 5 * giB
	vcdParam                       = "vcd"
	storageProfileParam            = "storageProfile"
	topologyKey                    = "topology.vcd.csi.fght.net/vcd"

	pubContextUnit     = "diskUnitNum"
	pubContextDiskSize = "diskSizeB"
)

var (
	supportedAccessMode = &csi.VolumeCapability_AccessMode{
		Mode: csi.VolumeCapability_AccessMode_SINGLE_NODE_WRITER,
	}
)

//we need to know vcd - aka region
//and storage profile
func (c *CsiDriver) CreateVolume(_ context.Context, req *csi.CreateVolumeRequest) (*csi.CreateVolumeResponse, error) {
	var vcd, storage string
	if req.Name == "" {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Name must be provided")
	}

	if req.VolumeCapabilities == nil || len(req.VolumeCapabilities) == 0 {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume capabilities must be provided")
	}

	if violations := validateCapabilities(req.VolumeCapabilities); len(violations) > 0 {
		return nil, status.Error(codes.InvalidArgument, fmt.Sprintf("volume capabilities cannot be satisified: %s", strings.Join(violations, "; ")))
	}
	if value, ok := req.Parameters[vcdParam]; !ok {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume parameter \""+vcdParam+"\" must be provided")
	} else {
		vcd = value
	}
	if value, ok := req.Parameters[storageProfileParam]; !ok {
		return nil, status.Error(codes.InvalidArgument, "CreateVolume Volume parameter \""+storageProfileParam+"\" must be provided")
	} else {
		storage = value
	}

	size, err := extractStorage(req.CapacityRange)
	if err != nil {
		return nil, status.Errorf(codes.OutOfRange, "invalid capacity range: %v", err)
	}

	resp := &csi.CreateVolumeResponse{Volume: &csi.Volume{
		CapacityBytes: size,
		VolumeId:      req.Name,
		AccessibleTopology: []*csi.Topology{
			{
				Segments: map[string]string{
					topologyKey: vcd,
				},
			},
		},
	}}

	err = c.vcl.CreateDisk(vcd, req.Name, storage, size)
	if err != nil {
		//disk exists
		if errors.Is(err, vcd_client.ErrDiskAlreadyExists) {
			c.l.Infof("disk already exists")
			return resp, nil
		}
		c.l.WithError(err).Errorf("cannot create disk")
		return nil, err
	}
	c.l.Infof("disk created")
	return resp, nil

}

func (c *CsiDriver) DeleteVolume(_ context.Context, req *csi.DeleteVolumeRequest) (*csi.DeleteVolumeResponse, error) {
	//check if
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "DeleteVolume Volume ID must be provided")
	}

	l := c.l.WithFields(logrus.Fields{
		"volume": req.VolumeId,
	})
	l.Info("delete volume called")

	err := c.vcl.DeleteDisk(req.VolumeId)
	if err != nil {
		l.WithError(err).Errorf("cannot delete volume")
		return nil, status.Error(codes.Internal, err.Error())
	}
	l.Infof("volume deleted")
	return &csi.DeleteVolumeResponse{}, nil
}

func (c *CsiDriver) ControllerPublishVolume(_ context.Context, req *csi.ControllerPublishVolumeRequest) (*csi.ControllerPublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume ID must be provided")
	}

	if req.NodeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Node ID must be provided")
	}

	if req.VolumeCapability == nil {
		return nil, status.Error(codes.InvalidArgument, "ControllerPublishVolume Volume capability must be provided")
	}

	if req.Readonly {
		return nil, status.Error(codes.AlreadyExists, "read only Volumes are not supported")
	}

	l := c.l.WithFields(logrus.Fields{
		"volume": req.VolumeId,
		"node":   req.NodeId,
	})
	l.Info("controller publish volume called")

	//TODO for resize add this option
	//if ok := d.resizeLocks.VolIdExists(req.VolumeId); ok {
	//	return nil, status.Errorf(codes.Internal, "Resize required, not publishing")
	//}

	//no we need to know is disk attached somewhere or not
	//this api must return attachement for volume
	//so we can add it to context
	diskAttach, err := c.vcl.AttachDisk(req.NodeId, req.VolumeId)
	if err != nil {
		switch err {
		case vcd_client.ErrDiskNotExists:
			l.Errorf("disk not exists, cannot attach")
			return nil, status.Error(codes.Internal, "disk not exists")
		case vcd_client.ErrVmNotFound:
			l.Errorf("vm not exists at cloud, cannot attach")
			return nil, status.Errorf(codes.NotFound, "vm not exists at cloud")
		case vcd_client.ErrWrongDiskAttach:
			return nil, status.Errorf(codes.FailedPrecondition,
				"Disk %q is attached to the wrong Instance, detach the Disk to fix it",
				req.VolumeId)
		default:
			l.WithError(err).Error("some internal error for disk attach")
			return nil, status.Error(codes.Internal, err.Error())
		}

	}

	l.Info("volume was attached")
	return &csi.ControllerPublishVolumeResponse{
		PublishContext: map[string]string{
			//for being able to guess which drive to mount, we need
			//capacity and unit number.
			pubContextDiskSize: diskAttach.Size,
			pubContextUnit:     diskAttach.Path,
		},
	}, nil
}

func (c *CsiDriver) ControllerUnpublishVolume(_ context.Context, req *csi.ControllerUnpublishVolumeRequest) (*csi.ControllerUnpublishVolumeResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ControllerUnpublishVolume Volume ID must be provided")
	}

	l := c.l.WithFields(logrus.Fields{
		"volume_id": req.VolumeId,
		"node_id":   req.NodeId,
		"method":    "controller_unpublish_volume",
	})
	l.Info("controller unpublish volume called")

	err := c.vcl.DetachDisk(req.NodeId, req.VolumeId)
	if err != nil {
		l.WithError(err).Error("cannot detach disk")
		return nil, status.Errorf(codes.Internal, err.Error())
	}
	l.Info("volume was detached")
	return &csi.ControllerUnpublishVolumeResponse{}, nil
}

func (c *CsiDriver) ValidateVolumeCapabilities(_ context.Context, req *csi.ValidateVolumeCapabilitiesRequest) (*csi.ValidateVolumeCapabilitiesResponse, error) {
	if req.VolumeId == "" {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities Volume ID must be provided")
	}

	if req.VolumeCapabilities == nil {
		return nil, status.Error(codes.InvalidArgument, "ValidateVolumeCapabilities Volume Capabilities must be provided")
	}

	l := c.l.WithFields(logrus.Fields{
		"volume_id":              req.VolumeId,
		"volume_capabilities":    req.VolumeCapabilities,
		"supported_capabilities": supportedAccessMode,
		"method":                 "validate_volume_capabilities",
	})
	l.Info("validate volume capabilities called")

	// if it's not supported (i.e: wrong region), we shouldn't override it
	resp := &csi.ValidateVolumeCapabilitiesResponse{
		Confirmed: &csi.ValidateVolumeCapabilitiesResponse_Confirmed{
			VolumeCapabilities: []*csi.VolumeCapability{
				{
					AccessMode: supportedAccessMode,
				},
			},
		},
	}

	l.WithField("confirmed", resp.Confirmed).Info("supported capabilities")
	return resp, nil
}

func (c *CsiDriver) ListVolumes(context.Context, *csi.ListVolumesRequest) (*csi.ListVolumesResponse, error) {
	resp := &csi.ListVolumesResponse{}

	volumes, err := c.vcl.ListVolumes()
	if err != nil {
		c.l.WithError(err).Errorf("cannot list volumes")
		return nil, err
	}
	c.l.Infof("listed volumes, len of result :%v", len(volumes))
	for _, volume := range volumes {
		ent := &csi.ListVolumesResponse_Entry{Volume: &csi.Volume{
			VolumeId:      volume.Name,
			CapacityBytes: volume.Cap,
		}}
		resp.Entries = append(resp.Entries, ent)
	}
	c.l.Infof("list executed")

	return resp, nil

}

func (c *CsiDriver) GetCapacity(context.Context, *csi.GetCapacityRequest) (*csi.GetCapacityResponse, error) {
	return nil, status.Error(codes.Unimplemented, "")

}

func (c *CsiDriver) ControllerGetCapabilities(context.Context, *csi.ControllerGetCapabilitiesRequest) (*csi.ControllerGetCapabilitiesResponse, error) {
	newCap := func(cap csi.ControllerServiceCapability_RPC_Type) *csi.ControllerServiceCapability {
		return &csi.ControllerServiceCapability{
			Type: &csi.ControllerServiceCapability_Rpc{
				Rpc: &csi.ControllerServiceCapability_RPC{
					Type: cap,
				},
			},
		}
	}

	var caps []*csi.ControllerServiceCapability
	for _, capability := range []csi.ControllerServiceCapability_RPC_Type{
		csi.ControllerServiceCapability_RPC_CREATE_DELETE_VOLUME,
		csi.ControllerServiceCapability_RPC_PUBLISH_UNPUBLISH_VOLUME,
		csi.ControllerServiceCapability_RPC_LIST_VOLUMES,
		csi.ControllerServiceCapability_RPC_EXPAND_VOLUME,
	} {
		caps = append(caps, newCap(capability))
	}

	resp := &csi.ControllerGetCapabilitiesResponse{
		Capabilities: caps,
	}

	c.l.WithFields(logrus.Fields{
		"response": resp,
		"method":   "controller_get_capabilities",
	}).Info("controller get capabilities called")
	return resp, nil
}

func (c *CsiDriver) CreateSnapshot(context.Context, *csi.CreateSnapshotRequest) (*csi.CreateSnapshotResponse, error) {
	return nil, errors.New("not implemeneted")
}

func (c *CsiDriver) DeleteSnapshot(context.Context, *csi.DeleteSnapshotRequest) (*csi.DeleteSnapshotResponse, error) {
	return nil, errors.New("not implemeneted")
}

func (c *CsiDriver) ListSnapshots(context.Context, *csi.ListSnapshotsRequest) (*csi.ListSnapshotsResponse, error) {
	return nil, errors.New("not implemeneted")
}

//TODO fix it
func (c *CsiDriver) ControllerExpandVolume(context.Context, *csi.ControllerExpandVolumeRequest) (*csi.ControllerExpandVolumeResponse, error) {
	return nil, errors.New("not implemeneted")
}

// extractStorage extracts the storage size in bytes from the given capacity
// range. If the capacity range is not satisfied it returns the default volume
// size. If the capacity range is below or above supported sizes, it returns an
// error.
func extractStorage(capRange *csi.CapacityRange) (int64, error) {
	if capRange == nil {
		return defaultVolumeSizeInBytes, nil
	}

	requiredBytes := capRange.GetRequiredBytes()
	requiredSet := 0 < requiredBytes
	limitBytes := capRange.GetLimitBytes()
	limitSet := 0 < limitBytes

	if !requiredSet && !limitSet {
		return defaultVolumeSizeInBytes, nil
	}

	if requiredSet && limitSet && limitBytes < requiredBytes {
		return 0, fmt.Errorf("limit (%v) can not be less than required (%v) size", formatBytes(limitBytes), formatBytes(requiredBytes))
	}

	if requiredSet && !limitSet && requiredBytes < minimumVolumeSizeInBytes {
		return 0, fmt.Errorf("required (%v) can not be less than minimum supported volume size (%v)", formatBytes(requiredBytes), formatBytes(minimumVolumeSizeInBytes))
	}

	if limitSet && limitBytes < minimumVolumeSizeInBytes {
		return 0, fmt.Errorf("limit (%v) can not be less than minimum supported volume size (%v)", formatBytes(limitBytes), formatBytes(minimumVolumeSizeInBytes))
	}

	if requiredSet && requiredBytes > maximumVolumeSizeInBytes {
		return 0, fmt.Errorf("required (%v) can not exceed maximum supported volume size (%v)", formatBytes(requiredBytes), formatBytes(maximumVolumeSizeInBytes))
	}

	if !requiredSet && limitSet && limitBytes > maximumVolumeSizeInBytes {
		return 0, fmt.Errorf("limit (%v) can not exceed maximum supported volume size (%v)", formatBytes(limitBytes), formatBytes(maximumVolumeSizeInBytes))
	}

	if requiredSet && limitSet && requiredBytes == limitBytes {
		return requiredBytes, nil
	}

	if requiredSet {
		return requiredBytes, nil
	}

	if limitSet {
		return limitBytes, nil
	}

	return defaultVolumeSizeInBytes, nil
}

func formatBytes(inputBytes int64) string {
	output := float64(inputBytes)
	unit := ""

	switch {
	case inputBytes >= tiB:
		output = output / tiB
		unit = "Ti"
	case inputBytes >= giB:
		output = output / giB
		unit = "Gi"
	case inputBytes >= miB:
		output = output / miB
		unit = "Mi"
	case inputBytes >= kiB:
		output = output / kiB
		unit = "Ki"
	case inputBytes == 0:
		return "0"
	}

	result := strconv.FormatFloat(output, 'f', 1, 64)
	result = strings.TrimSuffix(result, ".0")
	return result + unit
}

func validateCapabilities(caps []*csi.VolumeCapability) []string {
	violations := sets.NewString()
	for _, capability := range caps {
		if capability.GetAccessMode().GetMode() != supportedAccessMode.GetMode() {
			violations.Insert(fmt.Sprintf("unsupported access mode %s", capability.GetAccessMode().GetMode().String()))
		}

		accessType := capability.GetAccessType()
		switch accessType.(type) {
		case *csi.VolumeCapability_Block:
		case *csi.VolumeCapability_Mount:
		default:
			violations.Insert("unsupported access type")
		}
	}

	return violations.List()
}
