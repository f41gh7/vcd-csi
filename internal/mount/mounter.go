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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jaypipes/ghw"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type findmntResponse struct {
	FileSystems []fileSystem `json:"filesystems"`
}

type fileSystem struct {
	Target      string `json:"target"`
	Propagation string `json:"propagation"`
	FsType      string `json:"fstype"`
	Options     string `json:"options"`
}

type VolumeStatistics struct {
	AvailableBytes, TotalBytes, UsedBytes    int64
	AvailableInodes, TotalInodes, UsedInodes int64
}

const (
	// blkidExitStatusNoIdentifiers defines the exit code returned from blkid indicating that no devices have been found. See http://www.polarhome.com/service/man/?qf=blkid&tf=2&of=Alpinelinux for details.
	blkidExitStatusNoIdentifiers = 2
)

// Mounter is responsible for formatting and mounting volumes
type Mounter interface {
	// Format formats the source with the given filesystem type
	Format(source, fsType string) error

	// Mount mounts source to target with the given fstype and options.
	Mount(source, target, fsType string, options ...string) error

	// Unmount unmounts the given target
	Unmount(target string) error

	// IsFormatted checks whether the source device is formatted or not. It
	// returns true if the source device is already formatted.
	IsFormatted(source string) (bool, error)

	// IsMounted checks whether the target path is a correct mount (i.e:
	// propagated). It returns true if it's mounted. An error is returned in
	// case of system errors or if it's mounted incorrectly.
	IsMounted(target string) (bool, error)

	// GetStatistics returns capacity-related volume statistics for the given
	// volume path.
	GetStatistics(volumePath string) (VolumeStatistics, error)

	// IsBlockDevice checks whether the device at the path is a block device
	IsBlockDevice(volumePath string) (bool, error)


	//provides information about new disk location only by unit match
	GetDiskUnit(size string, unit string, bus string) (string, error)
}

type MounterHandler struct {
	log *logrus.Entry
}

// newMounter returns a new MounterHandler instance
func NewMounter(log *logrus.Entry) *MounterHandler {
	return &MounterHandler{
		log: log,
	}
}

func (m *MounterHandler) Format(source, fsType string) error {
	mkfsCmd := fmt.Sprintf("mkfs.%s", fsType)

	_, err := exec.LookPath(mkfsCmd)
	if err != nil {
		if err == exec.ErrNotFound {
			return fmt.Errorf("%q executable not found in $PATH", mkfsCmd)
		}
		return err
	}

	var mkfsArgs []string

	if fsType == "" {
		return errors.New("fs type is not specified for formatting the volume")
	}

	if source == "" {
		return errors.New("source is not specified for formatting the volume")
	}

	mkfsArgs = append(mkfsArgs, source)
	if fsType == "ext4" || fsType == "ext3" {
		mkfsArgs = []string{"-F", source}
	}

	m.log.WithFields(logrus.Fields{
		"cmd":  mkfsCmd,
		"args": mkfsArgs,
	}).Info("executing format command")

	out, err := exec.Command(mkfsCmd, mkfsArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("formatting disk failed: %v cmd: '%s %s' output: %q",
			err, mkfsCmd, strings.Join(mkfsArgs, " "), string(out))
	}

	return nil
}

func (m *MounterHandler) Mount(source, target, fsType string, opts ...string) error {
	var (
		mountCmd  = "mount"
		mountArgs []string
	)

	if source == "" {
		return errors.New("source is not specified for mounting the volume")
	}

	if target == "" {
		return errors.New("target is not specified for mounting the volume")
	}

	// This is a raw block device mount. Create the mount point as a file
	// since bind mount device node requires it to be a file
	if fsType == "" {
		// create directory for target, os.Mkdirall is noop if directory exists
		err := os.MkdirAll(filepath.Dir(target), 0750)
		if err != nil {
			return fmt.Errorf("failed to create target directory for raw block bind mount: %v", err)
		}

		file, err := os.OpenFile(target, os.O_CREATE, 0660)
		if err != nil {
			return fmt.Errorf("failed to create target file for raw block bind mount: %v", err)
		}
		_ = file.Close()
	} else {
		mountArgs = append(mountArgs, "-t", fsType)

		// create target, os.Mkdirall is noop if directory exists
		err := os.MkdirAll(target, 0750)
		if err != nil {
			return err
		}
	}

	if len(opts) > 0 {
		mountArgs = append(mountArgs, "-o", strings.Join(opts, ","))
	}

	mountArgs = append(mountArgs, source)
	mountArgs = append(mountArgs, target)

	m.log.WithFields(logrus.Fields{
		"cmd":  mountCmd,
		"args": mountArgs,
	}).Info("executing mount command")

	out, err := exec.Command(mountCmd, mountArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("mounting failed: %v cmd: '%s %s' output: %q",
			err, mountCmd, strings.Join(mountArgs, " "), string(out))
	}

	return nil
}

func (m *MounterHandler) Unmount(target string) error {
	umountCmd := "umount"
	if target == "" {
		return errors.New("target is not specified for unmounting the volume")
	}

	umountArgs := []string{target}

	m.log.WithFields(logrus.Fields{
		"cmd":  umountCmd,
		"args": umountArgs,
	}).Info("executing umount command")

	out, err := exec.Command(umountCmd, umountArgs...).CombinedOutput()
	if err != nil {
		return fmt.Errorf("unmounting failed: %v cmd: '%s %s' output: %q",
			err, umountCmd, target, string(out))
	}

	return nil
}

func (m *MounterHandler) IsFormatted(source string) (bool, error) {
	if source == "" {
		return false, errors.New("source is not specified")
	}

	blkidCmd := "blkid"
	_, err := exec.LookPath(blkidCmd)
	if err != nil {
		if err == exec.ErrNotFound {
			return false, fmt.Errorf("%q executable not found in $PATH", blkidCmd)
		}
		return false, err
	}

	blkidArgs := []string{source}

	m.log.WithFields(logrus.Fields{
		"cmd":  blkidCmd,
		"args": blkidArgs,
	}).Info("checking if source is formatted")

	exitCode := 0
	cmd := exec.Command(blkidCmd, blkidArgs...)
	err = cmd.Run()
	if err != nil {
		exitError, ok := err.(*exec.ExitError)
		if !ok {
			return false, fmt.Errorf("checking formatting failed: %v cmd: %q, args: %q", err, blkidCmd, blkidArgs)
		}
		ws := exitError.Sys().(syscall.WaitStatus)
		exitCode = ws.ExitStatus()
		if exitCode == blkidExitStatusNoIdentifiers {
			return false, nil
		} else {
			return false, fmt.Errorf("checking formatting failed: %v cmd: %q, args: %q", err, blkidCmd, blkidArgs)
		}
	}

	return true, nil
}

func (m *MounterHandler) IsMounted(target string) (bool, error) {
	if target == "" {
		return false, errors.New("target is not specified for checking the mount")
	}

	findmntCmd := "findmnt"
	_, err := exec.LookPath(findmntCmd)
	if err != nil {
		if err == exec.ErrNotFound {
			return false, fmt.Errorf("%q executable not found in $PATH", findmntCmd)
		}
		return false, err
	}

	findmntArgs := []string{"-o", "TARGET,PROPAGATION,FSTYPE,OPTIONS", "-M", target, "-J"}

	m.log.WithFields(logrus.Fields{
		"cmd":  findmntCmd,
		"args": findmntArgs,
	}).Info("checking if target is mounted")

	out, err := exec.Command(findmntCmd, findmntArgs...).CombinedOutput()
	if err != nil {
		// findmnt exits with non zero exit status if it couldn't find anything
		if strings.TrimSpace(string(out)) == "" {
			return false, nil
		}

		return false, fmt.Errorf("checking mounted failed: %v cmd: %q output: %q",
			err, findmntCmd, string(out))
	}

	// no response means there is no mount
	if string(out) == "" {
		return false, nil
	}

	var resp *findmntResponse
	err = json.Unmarshal(out, &resp)
	if err != nil {
		return false, fmt.Errorf("couldn't unmarshal data: %q: %s", string(out), err)
	}

	targetFound := false
	for _, fs := range resp.FileSystems {
		// check if the mount is propagated correctly. It should be set to shared.
		if fs.Propagation != "shared" {
			return true, fmt.Errorf("mount propagation for target %q is not enabled", target)
		}

		// the mountpoint should match as well
		if fs.Target == target {
			targetFound = true
		}
	}

	return targetFound, nil
}

func (m *MounterHandler) GetStatistics(volumePath string) (VolumeStatistics, error) {
	isBlock, err := m.IsBlockDevice(volumePath)
	if err != nil {
		return VolumeStatistics{}, fmt.Errorf("failed to determine if volume %s is block device: %v", volumePath, err)
	}

	if isBlock {
		// See http://man7.org/linux/man-pages/man8/blockdev.8.html for details
		output, err := exec.Command("blockdev", "getsize64", volumePath).CombinedOutput()
		if err != nil {
			return VolumeStatistics{}, fmt.Errorf("error when getting size of block volume at path %s: output: %s, err: %v", volumePath, string(output), err)
		}
		strOut := strings.TrimSpace(string(output))
		gotSizeBytes, err := strconv.ParseInt(strOut, 10, 64)
		if err != nil {
			return VolumeStatistics{}, fmt.Errorf("failed to parse size %s into int", strOut)
		}

		return VolumeStatistics{
			TotalBytes: gotSizeBytes,
		}, nil
	}

	var statfs unix.Statfs_t
	// See http://man7.org/linux/man-pages/man2/statfs.2.html for details.
	err = unix.Statfs(volumePath, &statfs)
	if err != nil {
		return VolumeStatistics{}, err
	}

	volStats := VolumeStatistics{
		AvailableBytes: int64(statfs.Bavail) * statfs.Bsize,
		TotalBytes:     int64(statfs.Blocks) * statfs.Bsize,
		UsedBytes:      (int64(statfs.Blocks) - int64(statfs.Bfree)) * statfs.Bsize,

		AvailableInodes: int64(statfs.Ffree),
		TotalInodes:     int64(statfs.Files),
		UsedInodes:      int64(statfs.Files) - int64(statfs.Ffree),
	}

	return volStats, nil
}

func (m *MounterHandler) IsBlockDevice(devicePath string) (bool, error) {
	var stat unix.Stat_t
	err := unix.Stat(devicePath, &stat)
	if err != nil {
		return false, err
	}

	return (stat.Mode & unix.S_IFMT) == unix.S_IFBLK, nil
}

//we assume, that volume size and disk unit cannot be the same (it`s very rare case
//so we filter all disks by it`s size
//then validate output by unit
//for paravirtual scsi it allways at last -1 part
//pci-0000:0b:00.0-scsi-0:0:0:0
//pci-0000:1b:00.0-scsi-0:0:5:0
//pci-0000:0b:00.0-scsi-0:0:11:0
//pci-0000:0b:00.0-scsi-0:0:6:0
//so we split by :
// [1] - must contain bus num or some kind of, dunno how to validte it
//[4] - contains unit number
func (m *MounterHandler) GetDiskgNameBySizeAndUnit(size string, unit string, bus string) (string, error) {
	l := m.log.WithFields(logrus.Fields{"size": size, "unit": unit, "bus": bus})
	parseUintSize, err := strconv.ParseUint(size, 10, 64)
	if err != nil {
		l.WithError(err).Errorf("cannot parse disk size to uint64")
		return "", err
	}
	block, err := ghw.Block()
	if err != nil {
		l.WithError(err).Errorf("cannot get information about disks at system")
		return "", err
	}
	matchedDisksBySize := make([]*ghw.Disk, 0)
	for _, disk := range block.Disks {
		l.Infof("disk: %v, bus: %s, type: %v, size: %v, controller: %v, path: %v", disk.Name, disk.BusType, disk.DriveType, disk.SizeBytes, disk.StorageController, disk.BusPath)
		if disk.SizeBytes == parseUintSize {
			l.Infof("disk matched by size, disk: %v, size: %v", disk.Name, disk.SizeBytes)
			matchedDisksBySize = append(matchedDisksBySize, disk)
		}
	}
	switch len(matchedDisksBySize) {
	case 0:
		//no disk found
		return "", errors.New("cannot find disk by size")
	case 1:
		//we are extremly lucky
		//validate
		//TODO recognize scsi/non scsi
		unitNum := extractUnitNumForScsi(matchedDisksBySize[0].BusPath)
		if unitNum != unit {
			l.Infof("disk matched by size, but unit number is not the same")
			return "", errors.New("disk matched by size, but unit num is not the same")

		}
		return "/dev/" + matchedDisksBySize[0].Name, nil
	default:
		//we need additional guess
		matchedBusUnit := make([]string, 0)
		for _, disk := range matchedDisksBySize {
			diskUnit := extractUnitNumForScsi(disk.BusPath)
			if diskUnit != unit {
				continue
			}

			matchedBusUnit = append(matchedBusUnit, disk.Name)
		}
		if len(matchedBusUnit) == 1 {
			return "/dev/" + matchedBusUnit[0], nil
		}
		return "", fmt.Errorf("disk not match by filter for unit number, matched result len: %v, but we want only 1 match", len(matchedBusUnit))
	}
}

//pci-0000: 1b :00.0-scsi-0 : 0 : 5 : 0
//pci-0000:0b:00.0-scsi-0:0:11:0
//must return 5 and 11
func extractUnitNumForScsi(bus string) string {
	splittedData := strings.Split(bus, ":")
	if len(splittedData) < 5 {
		return ""
	}
	return splittedData[4]

}

//we assume, that volume size and disk unit cannot be the same (it`s very rare case
//so we filter all disks by it`s size
//then validate output by unit
//for paravirtual scsi it allways at last -1 part
//pci-0000:0b:00.0-scsi-0:0:0:0
//pci-0000:1b:00.0-scsi-0:0:5:0
//pci-0000:0b:00.0-scsi-0:0:11:0
//pci-0000:0b:00.0-scsi-0:0:6:0
//so we split by :
// [1] - must contain bus num or some kind of, dunno how to validte it
//[4] - contains unit number
func (m *MounterHandler) GetDiskUnit(size string, unit string, bus string) (string, error) {
	l := m.log.WithFields(logrus.Fields{"size": size, "unit": unit, "bus": bus})
	block, err := ghw.Block()
	if err != nil {
		l.WithError(err).Errorf("cannot get information about disks at system")
		return "", err
	}
	matchedDisksByUnit := make([]*ghw.Disk, 0)
	for _, disk := range block.Disks {
		l.Infof("disk: %v, bus: %s, type: %v, size: %v, controller: %v, path: %v, all info: %v", disk.Name, disk.BusType, disk.DriveType, disk.SizeBytes, disk.StorageController, disk.BusPath,disk.String())
		unitNum := extractUnitNumForScsi(disk.BusPath)
		if unitNum == unit {
			l.Infof("disk matched by unit number")
			matchedDisksByUnit = append(matchedDisksByUnit, disk)
		}
	}
	switch len(matchedDisksByUnit) {
	case 0:
		//no disk found
		return "", errors.New("cannot find disk by size")
	case 1:
		return "/dev/" + matchedDisksByUnit[0].Name, nil
	default:
		//we need additional guess
		//TODO additional checks
		return "", fmt.Errorf("disk not match by filter for unit number, matched result len: %v, but we want only 1 match", len(matchedDisksByUnit))
	}
}
