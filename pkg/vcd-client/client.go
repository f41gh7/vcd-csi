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
	"errors"
	"fmt"
	"github.com/f41gh7/vcd-csi/conf"
	"github.com/sirupsen/logrus"
	"github.com/vmware/go-vcloud-director/v2/govcd"
	"github.com/vmware/go-vcloud-director/v2/types/v56"
	"net/url"
	"strconv"
	"sync"
)

type VcdClient struct {
	c          *conf.ControllerConfig
	l          *logrus.Entry
	VdcClients map[string]*govcd.Vdc
	mt         *sync.Mutex
	baseClient *govcd.VCDClient
}

var (
	ErrVmNotFound        = errors.New("vm not found at cloud")
	ErrWrongDiskAttach   = errors.New("disk attacheted to wrong vm")
	ErrDiskAlreadyExists = errors.New("disk already exists")
	ErrDiskNotExists     = errors.New("vm not exists")
	ErrNoFreeUnits       = errors.New("vm doest have free units for attach disk")
	ErrDiskAttachedToVm = errors.New("disk is attached to vm, cannot resize")
)

//Warning all methods should be called with refresh auth and mutex
// ITS STRICTLY RESTRICTED TO CALL ONE PUBLIC METHOD FROM ANOTHER
// IT WILL CAUSE  DEADLOCK
func NewVcdClient(c *conf.ControllerConfig, l *logrus.Entry) (*VcdClient, error) {
	l.Infof("initing new client")
	u, err := url.ParseRequestURI(c.CloudCredentails.Href)
	if err != nil {
		return nil, fmt.Errorf("unable to pass url: %s", err)
	}

	vcdclient := govcd.NewVCDClient(*u, c.CloudCredentails.Insecure)

	vc := &VcdClient{
		c:          c,
		l:          l,
		baseClient: vcdclient,
		mt:         &sync.Mutex{},
		VdcClients: map[string]*govcd.Vdc{},
	}
	if err := vc.buildClients(); err != nil {
		l.WithError(err).Errorf("cannot build client")
		return nil, err
	}

	return vc, nil
}

func (v *VcdClient) buildClients() error {
	v.l.Infof("building new clients")
	err := v.baseClient.Authenticate(v.c.CloudCredentails.User, v.c.CloudCredentails.Password, v.c.CloudCredentails.Org)
	if err != nil {
		v.l.WithError(err).Errorf("unable to authenticate: %s", err)
		return err
	}
	v.l.Infof("client inited")
	org, err := v.baseClient.GetOrgByNameOrId(v.c.CloudCredentails.Org)
	if err != nil {
		return err
	}
	for _, vdcName := range v.c.Vdcs {
		v.l.Infof("appending client for vdc: %v", vdcName)
		vdc, err := org.GetVDCByName(vdcName, true)
		if err != nil {
			return err
		}
		v.VdcClients[vdcName] = vdc

	}

	return nil
}

type VolumeWithCap struct {
	Name string
	Cap  int64
	VdcName string
}

type VcdService interface {
	CreateDisk(vdc, diskName, profile string, capacityBytes int64) error
	ListVolumes() ([]*VolumeWithCap, error)
	DeleteDisk(diskName string) error
	DetachDisk(vmName, diskName string) error
	AttachDisk(vmName, diskName string) (*DiskAttachParams, error)
	ResizeDisk(vdcName, diskName string, newDiskSize int64)  error
}

func (v *VcdClient) refreshAuth() error {
	v.l.Debugf("send auth response")
	response, err := v.baseClient.GetAuthResponse(v.c.CloudCredentails.User, v.c.CloudCredentails.Password, v.c.CloudCredentails.Org)
	if err != nil {
		return err
	}
	if response.StatusCode == 401 {
		v.l.Errorf("bad auth code, rebuilding clients")
		if err := v.buildClients(); err != nil {
			return err
		}
	}

	return nil
}

func (v *VcdClient) ListVolumes() ([]*VolumeWithCap, error) {
	v.l.Infof("exec list volume req")
	v.mt.Lock()
	defer v.mt.Unlock()

	err := v.refreshAuth()
	if err != nil {
		v.l.WithError(err).Error("cannot refresh auth")
		return nil, err
	}
	resp := make([]*VolumeWithCap, 0)
	for vdcName, vcd := range v.VdcClients {
		err := vcd.Refresh()
		if err != nil {
			v.l.WithError(err).Errorf("cannot refresh vcd: %v", vdcName)
			return nil, err

		}
		for _, ent := range vcd.Vdc.ResourceEntities {
			for _, obj := range ent.ResourceEntity {
				v.l.Debugf("found object name: %v,type :%v, href: %v", obj.Name, obj.Type, obj.HREF)
				//find disk by hreaf
				if obj.Type == types.MimeDisk {
					d, err := vcd.GetDiskByHref(obj.HREF)
					if err != nil {
						v.l.WithError(err).Errorf("error getting disk by href")
						return nil, err
					}
					v.l.Infof("found volume: %v, cap: %v", obj.Name, d.Disk.Size)
					resp = append(resp, &VolumeWithCap{
						Name: obj.Name,
						Cap: d.Disk.Size,
						VdcName: vdcName})
				}
			}
		}
	}
	v.l.Infof("listed volumes, count: %v",len(resp))
	return resp, nil
}

//we must validate min size before calling this func
func (v *VcdClient) CreateDisk(vdcName, diskName, profile string, capacityBytes int64) error {
	l := v.l.WithFields(logrus.Fields{"vdcName": vdcName, "diskName": diskName, "profile": profile, "size": capacityBytes})
	l.Infof("creating new disk")
	v.mt.Lock()
	defer v.mt.Unlock()

	err := v.refreshAuth()
	if err != nil {
		v.l.WithError(err).Error("cannot refresh auth")
		return err
	}

	if client, ok := v.VdcClients[vdcName]; ok {
		err := client.Refresh()
		if err != nil {
			v.l.WithError(err).Errorf("cannot refresh vdcName: %v", vdcName)
			return err
		}

		exists, err := v.disksExists(client, diskName)
		if err != nil {
			l.WithError(err).Errorf("cannot check if disk exists")
			return err
		}
		if exists {
			l.Infof("disk already exists, nothing to do")
			return nil
		}

		//lets get storage profile
		StorageReference, err := client.FindStorageProfileReference(profile)
		if err != nil {
			l.WithError(err).Errorf("cannot find profile for vdcName")
			return err
		}
		task, err := client.CreateDisk(&types.DiskCreateParams{Disk: &types.Disk{
			Name:           diskName,
			Size:           capacityBytes,
			StorageProfile: &StorageReference,
			BusType:        v.c.DefaultBusType,
			BusSubType:     v.c.DefaultSubBusType,
		}})
		if err != nil {
			l.WithError(err).Errorf("cannot create disk")
		}
		l.Infof("waiting for disk creation completion")
		err = task.WaitTaskCompletion()
		if err != nil {
			l.WithError(err).Errorf("cannot wait for disk creation")
			return err
		}
		l.Infof("created disk")
		return nil
	}
	l.Errorf("vdcName not found: %v", vdcName)
	return errors.New("vdcName not found: " + vdcName)
}

func (v *VcdClient) DeleteDisk(diskName string) error {
	l := v.l.WithField("disk", diskName)
	l.Infof("deleting disk")

	v.mt.Lock()
	defer v.mt.Unlock()

	err := v.refreshAuth()
	if err != nil {
		v.l.WithError(err).Error("cannot refresh auth")
		return err
	}

	//we have to guess very disk is located
	for vcdName, client := range v.VdcClients {
		l := l.WithField("vcd_name", vcdName)
		err := client.Refresh()
		if err != nil {
			v.l.WithError(err).Errorf("cannot refresh vcd: %v", vcdName)
			return err
		}

		disk, err := getDiskByName(client, diskName, l)
		if err != nil {
			l.WithError(err).Errorf("cannot find disk for removal at vcd")
			continue
		}
		vmAttach, err := disk.AttachedVM()
		if err != nil {
			l.WithError(err).Errorf("cannot check has disk vm attach or not")
			return err
		}
		if vmAttach != nil {
			l.Infof("disk is attached to vm, detaching it first vm: %v", vmAttach.Name)
			err := v.detachDisk(client, l, diskName)
			if err != nil {
				l.WithError(err).Errorf("cannot detach disk")
				return err
			}
			l.Infof("disk was detached from vm")
		}

		l.Infof("deleting disk")
		task, err := disk.Delete()
		if err != nil {
			l.WithError(err).Errorf("cannot delete disk")
			return err

		}
		l.Infof("waiting for disk deletion")
		err = task.WaitTaskCompletion()
		if err != nil {
			l.WithError(err).Errorf("cannot wait for disk delete complete")
			return err
		}
		l.Infof("disk was deleted")
		return nil
	}
	l.Infof("seems like disk doesnt exists")
	return nil
}

func (v *VcdClient) DetachDisk(vm, VolumeName string) error {
	l := v.l.WithFields(logrus.Fields{"vm": vm, "disk": VolumeName})
	l.Infof("detachhing disk")
	v.mt.Lock()
	defer v.mt.Unlock()

	err := v.refreshAuth()
	if err != nil {
		v.l.WithError(err).Error("cannot refresh auth")
		return err
	}

	for vcdName, client := range v.VdcClients {
		l := l.WithField("vcd_name", vcdName)
		err := client.Refresh()
		if err != nil {
			v.l.WithError(err).Errorf("cannot refresh vcd: %v", vcdName)
			return err
		}

		err = v.detachDisk(client, l, VolumeName)
		if err != nil {
			if errors.Is(err, govcd.ErrorEntityNotFound) {
				continue
			}
			l.WithError(err).Error("cannot detach disk")
			return err
		}
		l.Infof("disk detached")
		return nil

	}
	l.Warnf("disk wasnt found, probably it doesnt exists, seems like bug or not")
	return nil
}

type DiskAttachParams struct {
	Name string
	Path string
	Size string
}

func (v *VcdClient) AttachDisk(vm, disk string) (*DiskAttachParams, error) {
	l := v.l.WithFields(logrus.Fields{"vm": vm, "disk": disk})
	l.Infof("attaching disk")
	v.mt.Lock()
	defer v.mt.Unlock()

	err := v.refreshAuth()
	if err != nil {
		v.l.WithError(err).Error("cannot refresh auth")
		return nil, err
	}
	for vcdName, client := range v.VdcClients {
		l := l.WithField("vcd_name", vcdName)
		err := client.Refresh()
		if err != nil {
			v.l.WithError(err).Errorf("cannot refresh vcd: %v", vcdName)
			return nil, err
		}
		l.Infof("search vm by name")
		vappVm, err := v.getVMByName(client, vm)
		if err != nil {
			l.WithError(err).Errorf("cannot find vm for disk attach in vdc, lets try next")
			if errors.Is(err, ErrVmNotFound) {
				continue
			}
			l.WithError(err).Errorf("cannot query vm")
			return nil, err
		}
		Disk, err := getDiskByName(client, disk, l)
		if err != nil {
			//disk not exists
			if errors.Is(err, govcd.ErrorEntityNotFound) {
				l.WithError(err).Errorf("disk not exists")
				return nil, ErrDiskNotExists
			}
			l.WithError(err).Errorf("cannot get disk by name for attaching")
			return nil, err
		}
		l.Infof("check if disk is attached to vm")
		attachedVM, err := Disk.AttachedVM()
		if err != nil {
			l.WithError(err).Errorf("cannot get attaching info for disk")
			return nil, err
		}
		//check if disk already attached
		if attachedVM != nil {
			l.WithField("vm_attach_name", attachedVM.Name)
			l.Infof("disk already attached to vm, check if it is correct")
			//probably it`s our vm
			if vappVm.VM.HREF == attachedVM.HREF {
				//disk already attached to this vm
				l.Infof("disk already attached to correct vm")
				for _, diskSpec := range vappVm.VM.VmSpecSection.DiskSection.DiskSettings {
					//aat vm speck disk may be nil
					if diskSpec.Disk == nil {
						continue
					}
					if diskSpec.Disk.HREF == Disk.Disk.HREF {
						//we found correct attachemtnet
						convSize := strconv.FormatInt(Disk.Disk.Size, 10)
						l.Infof("found disk attached to vm, name: %s, size: %v", diskSpec.Disk.Name, Disk.Disk.Size)
						return &DiskAttachParams{
							Name: disk,
							//we need data convertion to string for publish context
							Path: strconv.Itoa(diskSpec.UnitNumber),
							Size: convSize,
						}, nil
					}
				}
				//thats strange, seems like bug
				l.Warnf("cannot find disk spec at vm setting, seems like bug")
			}

			//do we really need it?
			//it`s error prone
			l.Infof("detaching disk from incorrect vm")
			err := v.detachDisk(client, l, disk)
			if err != nil {
				l.Errorf("disk attached to wrong vm, detach it")
				return nil, ErrWrongDiskAttach
			}
			l.Infof("disk was detached from vm incorrect vm")
		}

		l = l.WithField("vm_attach_name", vm)

		l.Infof("attaching disk to vm")

		unitNum, err := getFreeUnitNum(vappVm, v.c.DefaultBusNum)
		task, err := vappVm.AttachDisk(&types.DiskAttachOrDetachParams{Disk: &types.Reference{
			Type: Disk.Disk.Type,
			Name: Disk.Disk.Name,
			HREF: Disk.Disk.HREF},
			BusNumber:  &v.c.DefaultBusNum,
			UnitNumber: &unitNum,
		})
		if err != nil {
			l.WithError(err).Errorf("cannot attach disk to vm")
			return nil, err
		}
		err = task.WaitTaskCompletion()
		if err != nil {
			l.WithError(err).Errorf("cannot wait for disk attach completion")
			return nil, err
		}
		convSize := strconv.FormatInt(Disk.Disk.Size, 10)
		l.Infof("disk attached correctly")
		return &DiskAttachParams{
					Name: disk,
					Path: strconv.Itoa(unitNum),
					Size: convSize,
				}, nil}


	l.WithError(ErrVmNotFound).Errorf("cannot find vm for disk attach its not possible")
	return nil, ErrVmNotFound

}

func (v *VcdClient) ResizeDisk(vdcName, diskName string, newDiskSize int64)  error {
	l := v.l.WithFields(logrus.Fields{
		"disk": diskName,
		"vdc": vdcName,
	})
	l.Infof("starting disk resize")
	v.mt.Lock()
	defer v.mt.Unlock()

	err := v.refreshAuth()
	if err != nil {
		v.l.WithError(err).Error("cannot refresh auth")
		return  err
	}
	if client,ok := v.VdcClients[vdcName];ok {
		l.Infof("search disk for resize")
		disk, err := getDiskByName(client, diskName, l)
		if err != nil {
			l.WithError(err).Error("cannot get disk")
			return err
		}
		vm, err := disk.AttachedVM()
		if err != nil {
			l.WithError(err).Errorf("cannot get information about disk attachement for resize")
			return err
		}
		if vm != nil {
			l.WithError(ErrDiskAttachedToVm).Errorf("disk already attached to vm: %v",vm.Name)
			return ErrDiskAttachedToVm
		}
		task, err := disk.Update(&types.Disk{Name:disk.Disk.Name,Size:newDiskSize})
		if err != nil {
			l.WithError(err).Error("cannot update disk")
			return err
		}
		l.Infof("wait for task completion")
		err = task.WaitTaskCompletion()
		if err != nil {
			l.WithError(err).Error("cannot wait for task completion")
			return err
		}
		l.Infof("disk resized successfully")
		return nil
	}

    return errors.New("vdc not exists")
}

//helper method
func (v *VcdClient) detachDisk(client *govcd.Vdc, l *logrus.Entry, VolumeName string) error {
	l.Infof("search at vdc")
	disk, err := getDiskByName(client, VolumeName, l)
	if err != nil {
		return err
	}
	attachedVM, err := disk.AttachedVM()
	if err != nil {
		l.WithError(err).Errorf("cannot get attached vm")
		return err
	}
	if attachedVM == nil {
		//nothing to do disk detached
		l.Infof("disk already detached")
		return nil
	}
	//it`s safe to call, we pass client to it
	vappVm, err := v.getVMByName(client, attachedVM.Name)
	if err != nil {
		l.WithError(err).Errorf("cannot find vm for attached disk, its bug,  vm: %v", attachedVM.Name)
		return err
	}
	for _, diskSpec := range vappVm.VM.VmSpecSection.DiskSection.DiskSettings {
		if diskSpec.Disk == nil {
			continue
		}
		if diskSpec.Disk.HREF == disk.Disk.HREF {
			l.Infof("found matched disk, lets detach it")
			task, err := vappVm.DetachDisk(&types.DiskAttachOrDetachParams{Disk: &types.Reference{
				Type: diskSpec.Disk.Type,
				Name: diskSpec.Disk.Name,
				HREF: diskSpec.Disk.HREF},
				BusNumber: &diskSpec.BusNumber, UnitNumber: &diskSpec.UnitNumber})
			if err != nil {
				l.WithError(err).Errorf("cannot deattach disk from vm")
				return err
			}
			err = task.WaitTaskCompletion()
			if err != nil {
				l.WithError(err).Errorf("cannot wait for disk deattach completion")
				return err
			}
			l.Infof("detached disk")
			return nil

		}
	}
	l.Warnf("cannot find disk at vm spec, is it bug?")
	return nil
}

//auth do not need
func (v *VcdClient) getVMByName(client *govcd.Vdc, vm string) (*govcd.VM, error) {
	v.l.Infof("get vm by name: %v", vm)
	for _, ent := range client.Vdc.ResourceEntities {
		for _, obj := range ent.ResourceEntity {
			v.l.Debugf("entity name: %v, type: %v", obj.Name, obj.Type)
			if obj.Type == types.MimeVApp {
				vApp, err := client.GetVAppByName(obj.Name, false)
				if err != nil {
					v.l.WithError(err).Errorf("cannot query vapp")
					return nil, err
				}
				vappVm, err := vApp.GetVMByName(vm, false)
				if err != nil {
					if errors.Is(err, govcd.ErrorEntityNotFound) {
						v.l.Debugf("entity not found")
						continue
					}
					v.l.WithError(err).Errorf("cannot get vm by name")
					return nil, err
				}
				v.l.Infof("vm found at vcd: %v", vappVm.VM.Name)
				return vappVm, nil

			}
		}

	}
	return nil, ErrVmNotFound
}

func getDiskByName(client *govcd.Vdc, diskName string, l *logrus.Entry) (*govcd.Disk, error) {

	disks, err := client.GetDisksByName(diskName, true)
	if err != nil {
		l.WithError(err).Error("cannot get disk from vdc")
		return nil, err
	}
	if len(*disks) != 1 {
		return nil, errors.New("disk count mismatch, we want excatly one, seems like collision, count: " + string(len(*disks)))
	}
	disk := (*disks)[0]

	l.Infof("disk was found, disk: %s, size: %d", disk.Disk.Name, disk.Disk.Size)
	return &disk, nil
}

func (v *VcdClient) disksExists(client *govcd.Vdc, diskName string) (bool, error) {
	l := v.l.WithField("disk", diskName)
	l.Infof("checking if disk exists")
	d, err := client.GetDisksByName(diskName, false)
	if err != nil {
		if errors.Is(err, govcd.ErrorEntityNotFound) {
			return false, nil
		}
		l.WithError(err).Errorf("error listing disks")
		return false, err
	}
	if len(*d) == 1 {
		d1 := (*d)[0]
		l.Infof("exists disk info, name: %s, size: %v", d1.Disk.Name, d1.Disk.Size)
	}
	l.Infof("disk exists")
	return true, nil

}

//ok, lets reserve 8-15 units for our disks
func buildDiskUnits() map[int]bool {
	units := map[int]bool{}
	for i := 8; i < 16; i++ {
		units[i] = false
	}
	return units
}

func getFreeUnitNum(vm *govcd.VM, busNum int) (int, error) {
	unitsInUse := buildDiskUnits()
	for _, diskSetting := range vm.VM.VmSpecSection.DiskSection.DiskSettings {
		if diskSetting != nil {
			if diskSetting.BusNumber == busNum {
				//we have matched bus
				unitsInUse[diskSetting.UnitNumber] = true
			}
		}
	}
	//no we pick first not in use
	for k, v := range unitsInUse {
		if !v {
			return k, nil
		}
	}
	//no free units...
	return 0, ErrNoFreeUnits
}
