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
	"github.com/osbuild/images/pkg/osbuild"
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

type otkGenPartitionInput struct {
	TotalSize uint64 `json:"total_size"`
}

type otkGenPartitionsOutput struct {
	PartitionTable *disk.PartitionTable `json:"internal-partition-table"`
	KernelOptsList []string             `json:"kernel_opts_list"`
}

func run() error {
	var genPartInput otkGenPartitionInput
	if err := json.NewDecoder(os.Stdin).Decode(&genPartInput); err != nil {
		return err
	}

	rngSeed, err := cmdutil.SeedArgFor(&buildconfig.BuildConfig{}, "", "", "")
	if err != nil {
		return err
	}
	source := rand.NewSource(rngSeed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	pt, err := disk.NewPartitionTable(&basePt, nil, genPartInput.TotalSize, disk.DefaultPartitioningMode, nil, rng)
	if err != nil {
		return err
	}

	kernelOptions := osbuild.GenImageKernelOptions(pt)
	otkPart := otkGenPartitionsOutput{
		PartitionTable: pt,
		KernelOptsList: kernelOptions,
	}
	ptJson, err := json.Marshal(otkPart)
	if err != nil {
		return fmt.Errorf("failed to martial partition table: %w\n", err)
	}

	fmt.Printf("%s\n", ptJson)
	return nil
}

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err.Error())
		os.Exit(1)
	}
}
