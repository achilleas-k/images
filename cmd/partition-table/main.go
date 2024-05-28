package main

import (
	"encoding/json"
	"fmt"
	"math/rand"
	"os"

	"github.com/osbuild/images/internal/buildconfig"
	"github.com/osbuild/images/internal/cmdutil"
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/disk"
)

var basePt = disk.PartitionTable{
	UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
	Type: "gpt",
	Partitions: []disk.Partition{
		{
			Size:     1 * common.MebiByte,
			Bootable: true,
			Type:     disk.BIOSBootPartitionGUID,
			UUID:     disk.BIOSBootPartitionUUID,
		},
		{
			Size: 200 * common.MebiByte,
			Type: disk.EFISystemPartitionGUID,
			UUID: disk.EFISystemPartitionUUID,
			Payload: &disk.Filesystem{
				Type:         "vfat",
				UUID:         disk.EFIFilesystemUUID,
				Mountpoint:   "/boot/efi",
				Label:        "EFI-SYSTEM",
				FSTabOptions: "defaults,uid=0,gid=0,umask=077,shortname=winnt",
				FSTabFreq:    0,
				FSTabPassNo:  2,
			},
		},
		{
			Size: 500 * common.MebiByte,
			Type: disk.FilesystemDataGUID,
			UUID: disk.FilesystemDataUUID,
			Payload: &disk.Filesystem{
				Type:         "ext4",
				Mountpoint:   "/boot",
				Label:        "boot",
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		},
		{
			Size: 2 * common.GibiByte,
			Type: disk.FilesystemDataGUID,
			UUID: disk.RootPartitionUUID,
			Payload: &disk.Filesystem{
				Type:         "ext4",
				Label:        "root",
				Mountpoint:   "/",
				FSTabOptions: "defaults",
				FSTabFreq:    0,
				FSTabPassNo:  0,
			},
		},
	},
}

// Serializable version of the partition table that is used for formatting the
// output of this command.
type partitionTable struct {
	Size       uint64 `json:"size"`
	UUID       string `json:"uuid"`
	Type       string `json:"type"`
	SectorSize uint64 `json:"sector_size"`

	Partitions []partition
}

type partition struct {
	Start    uint64 `json:"start"`
	Size     uint64 `json:"size"`
	Type     string `json:"type"`
	Bootable bool   `json:"bootable"`

	UUID string `json:"uuid"`
}

func main() {
	rngSeed, err := cmdutil.SeedArgFor(&buildconfig.BuildConfig{}, "", "", "")
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}
	source := rand.NewSource(rngSeed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	pt, err := disk.NewPartitionTable(&basePt, nil, 0, disk.DefaultPartitioningMode, nil, rng)
	if err != nil {
		fmt.Fprintf(os.Stderr, err.Error())
		os.Exit(1)
	}

	ptJson, err := json.Marshal(pt)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to martial partition table: %s\n", err.Error())
	}
	fmt.Println(string(ptJson))
}
