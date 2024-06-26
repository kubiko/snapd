// -*- Mode: Go; indent-tabs-mode: t -*-

/*
 * Copyright (C) 2019-2020 Canonical Ltd
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License version 3 as
 * published by the Free Software Foundation.
 *
 * This program is distributed in the hope that it will be useful,
 * but WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 * GNU General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program.  If not, see <http://www.gnu.org/licenses/>.
 *
 */

package install

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/snapcore/snapd/dirs"
	"github.com/snapcore/snapd/gadget"
	"github.com/snapcore/snapd/gadget/quantity"
	"github.com/snapcore/snapd/kernel"
	"github.com/snapcore/snapd/logger"
	"github.com/snapcore/snapd/osutil/mkfs"
	"github.com/snapcore/snapd/snap"
)

var (
	mkfsImpl                      = mkfs.Make
	kernelEnsureKernelDriversTree = kernel.EnsureKernelDriversTree
)

// KernelSnapInfo includes information from the kernel snap that is
// needed to build a drivers tree. Defin
type KernelSnapInfo struct {
	Name     string
	Revision snap.Revision
	// MountPoint is the root of the files from the kernel snap
	MountPoint string
	// NeedsDriversTree will be set if a drivers tree needs to be
	// build on installation
	NeedsDriversTree bool
	// IsCore is set if this is UC
	IsCore bool
}

type mkfsParams struct {
	Type       string
	Device     string
	Label      string
	Size       quantity.Size
	SectorSize quantity.Size
}

// makeFilesystem creates a filesystem on the on-disk structure, according
// to the filesystem type defined in the gadget. If sectorSize is specified,
// that sector size is used when creating the filesystem, otherwise if it is
// zero, automatic values are used instead.
func makeFilesystem(params mkfsParams) error {
	logger.Debugf("create %s filesystem on %s with label %q", params.Type, params.Device, params.Label)
	if err := mkfsImpl(params.Type, params.Device, params.Label, params.Size, params.SectorSize); err != nil {
		return err
	}
	return udevTrigger(params.Device)
}

// mountFilesystem mounts the filesystem on a given device with
// filesystem type fs under the provided mount point directory.
func mountFilesystem(fsDevice, fs, mountpoint string) error {
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		return fmt.Errorf("cannot create mountpoint: %v", err)
	}
	if err := sysMount(fsDevice, mountpoint, fs, 0, ""); err != nil {
		return fmt.Errorf("cannot mount filesystem %q at %q: %v", fsDevice, mountpoint, err)
	}

	return nil
}

func unmountWithFallbackToLazy(mntPt, operationMsg string) error {
	if err := sysUnmount(mntPt, 0); err != nil {
		logger.Noticef("cannot unmount %s after %s: %v (trying lazy unmount next)", mntPt, operationMsg, err)
		// lazy umount on error, see LP:2025402
		if err = sysUnmount(mntPt, syscall.MNT_DETACH); err != nil {
			logger.Noticef("cannot lazy unmount %q: %v", mntPt, err)
			return err
		}
	}
	return nil
}

// writeContent populates the given on-disk filesystem structure with a
// corresponding filesystem device, according to the contents defined in the
// gadget.
func writeFilesystemContent(laidOut *gadget.LaidOutStructure, kSnapInfo *KernelSnapInfo, fsDevice string, observer gadget.ContentObserver) (err error) {
	mountpoint := filepath.Join(dirs.SnapRunDir, "gadget-install", strings.ReplaceAll(strings.Trim(fsDevice, "/"), "/", "-"))
	if err := os.MkdirAll(mountpoint, 0755); err != nil {
		return err
	}

	// temporarily mount the filesystem
	logger.Debugf("mounting %q in %q (fs type %q)", fsDevice, mountpoint, laidOut.Filesystem())
	if err := sysMount(fsDevice, mountpoint, laidOut.Filesystem(), 0, ""); err != nil {
		return fmt.Errorf("cannot mount %q at %q: %v", fsDevice, mountpoint, err)
	}
	defer func() {
		errUnmount := unmountWithFallbackToLazy(mountpoint, "writing filesystem content")
		if err == nil && errUnmount != nil {
			err = fmt.Errorf("cannot unmount %v after writing filesystem content: %v", fsDevice, errUnmount)
		}
	}()
	fs, err := gadget.NewMountedFilesystemWriter(nil, laidOut, observer)
	if err != nil {
		return fmt.Errorf("cannot create filesystem image writer: %v", err)
	}

	var noFilesToPreserve []string
	if err := fs.Write(mountpoint, noFilesToPreserve); err != nil {
		return fmt.Errorf("cannot create filesystem image: %v", err)
	}

	// For data partition, build drivers tree if required, so kernel
	// drivers are available on first boot of the installed system.
	if laidOut.Role() == gadget.SystemData && kSnapInfo != nil && kSnapInfo.NeedsDriversTree {
		destRoot := mountpoint
		if kSnapInfo.IsCore {
			destRoot = filepath.Join(mountpoint, "system-data")
		}
		destDir := kernel.DriversTreeDir(destRoot, kSnapInfo.Name, kSnapInfo.Revision)
		logger.Noticef("building drivers tree in %s", destDir)

		if err := kernelEnsureKernelDriversTree(kSnapInfo.MountPoint, destDir, nil,
			&kernel.KernelDriversTreeOptions{KernelInstall: true}); err != nil {
			return err
		}
	}

	return nil
}
