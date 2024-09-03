package disk_test

import (
	"math/rand"
	"testing"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/testdisk"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/disk"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPartitionTable_GetMountpointSize(t *testing.T) {
	pt := testdisk.MakeFakePartitionTable("/", "/app")

	size, err := pt.GetMountpointSize("/")
	assert.NoError(t, err)
	assert.Equal(t, testdisk.FakePartitionSize, size)

	size, err = pt.GetMountpointSize("/app")
	assert.NoError(t, err)
	assert.Equal(t, testdisk.FakePartitionSize, size)

	// non-existing
	_, err = pt.GetMountpointSize("/custom")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot find mountpoint /custom")
}

func TestPartitionTable_GenerateUUIDs(t *testing.T) {
	pt := disk.PartitionTable{
		Type: "gpt",
		Partitions: []disk.Partition{
			{
				Size:     1 * common.MebiByte,
				Bootable: true,
				Type:     disk.BIOSBootPartitionGUID,
				UUID:     disk.BIOSBootPartitionUUID,
			},
			{
				Size: 2 * common.GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					// create mixed xfs root filesystem and a btrfs /var partition
					Type:         "xfs",
					Label:        "root",
					Mountpoint:   "/",
					FSTabOptions: "defaults",
					FSTabFreq:    0,
					FSTabPassNo:  0,
				},
			},
			{
				Size: 10 * common.GibiByte,
				Payload: &disk.Btrfs{
					Subvolumes: []disk.BtrfsSubvolume{
						{
							Mountpoint: "/var",
						},
					},
				},
			},
		},
	}

	// Static seed for testing
	/* #nosec G404 */
	rnd := rand.New(rand.NewSource(0))

	pt.GenerateUUIDs(rnd)

	// Check that GenUUID doesn't change already defined UUIDs
	assert.Equal(t, disk.BIOSBootPartitionUUID, pt.Partitions[0].UUID)

	// Check that GenUUID generates fresh UUIDs if not defined prior the call
	assert.Equal(t, "a178892e-e285-4ce1-9114-55780875d64e", pt.Partitions[1].UUID)
	assert.Equal(t, "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75", pt.Partitions[1].Payload.(*disk.Filesystem).UUID)

	// Check that GenUUID generates the same UUID for BTRFS and its subvolumes
	assert.Equal(t, "fb180daf-48a7-4ee0-b10d-394651850fd4", pt.Partitions[2].Payload.(*disk.Btrfs).UUID)
	assert.Equal(t, "fb180daf-48a7-4ee0-b10d-394651850fd4", pt.Partitions[2].Payload.(*disk.Btrfs).Subvolumes[0].UUID)
}

func TestPartitionTable_GenerateUUIDs_VFAT(t *testing.T) {
	pt := disk.PartitionTable{
		Type: "dos",
		Partitions: []disk.Partition{
			{
				Size: 2 * common.GibiByte,
				Type: disk.FilesystemDataGUID,
				Payload: &disk.Filesystem{
					Type:       "vfat",
					Mountpoint: "/boot/efi",
				},
			},
		},
	}

	// Static seed for testing
	/* #nosec G404 */
	rnd := rand.New(rand.NewSource(0))

	pt.GenerateUUIDs(rnd)

	assert.Equal(t, "6e4ff95f", pt.Partitions[0].Payload.(*disk.Filesystem).UUID)
}

func TestNewCustomPartitionTable(t *testing.T) {
	// Static seed for testing
	/* #nosec G404 */
	rnd := rand.New(rand.NewSource(0))
	require := require.New(t)

	type testCase struct {
		customizations *blueprint.PartitioningCustomization
		expected       *disk.PartitionTable
	}

	testCases := map[string]testCase{
		"null": {
			customizations: nil,
			expected:       &disk.PartitionTable{},
		},
		"plain": {
			customizations: &blueprint.PartitioningCustomization{
				Plain: &blueprint.PlainFilesystemCustomization{
					Mountpoints: []blueprint.MountpointCustomization{
						{
							Mountpoint: "/data",
							MinSize:    20,
							Label:      "data",
							Type:       "ext4",
						},
					},
				},
			},
			expected: &disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     20,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "data",
							Mountpoint:   "/data",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"plain+": {
			customizations: &blueprint.PartitioningCustomization{
				Plain: &blueprint.PlainFilesystemCustomization{
					Mountpoints: []blueprint.MountpointCustomization{
						{
							Mountpoint: "/",
							MinSize:    50,
							Label:      "root",
							Type:       "xfs",
						},
						{
							Mountpoint: "/home",
							MinSize:    20,
							Label:      "home",
							Type:       "ext4",
						},
					},
				},
			},
			expected: &disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     50,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "root",
							Mountpoint:   "/",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    0,
						Size:     20,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "ext4",
							Label:        "home",
							Mountpoint:   "/home",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
				},
			},
		},
		"lvm": {
			customizations: &blueprint.PartitioningCustomization{
				LVM: &blueprint.LVMCustomization{
					VolumeGroups: []blueprint.VGCustomization{
						{
							Name:    "testvg",
							MinSize: 100,
							LogicalVolumes: []blueprint.LVCustomization{
								{
									Name: "varloglv",
									MountpointCustomization: blueprint.MountpointCustomization{
										Mountpoint: "/var/log",
										MinSize:    10,
										Label:      "var-log",
										Type:       "xfs",
									},
								},
								{
									Name: "rootlv",
									MountpointCustomization: blueprint.MountpointCustomization{
										Mountpoint: "/",
										MinSize:    50,
										Label:      "root",
										Type:       "xfs",
									},
								},
							},
						},
					},
				},
			},
			expected: &disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     512 * common.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    0,
						Size:     100,
						Type:     disk.LVMPartitionGUID,
						Bootable: false,
						Payload: &disk.LVMVolumeGroup{
							Name:        "testvg",
							Description: "created via lvm2 and osbuild",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Size: 10,
									Payload: &disk.Filesystem{
										Label:        "var-log",
										Type:         "xfs",
										Mountpoint:   "/var/log",
										FSTabOptions: "defaults",
									},
								},
								{
									Name: "rootlv",
									Size: 50,
									Payload: &disk.Filesystem{
										Label:        "root",
										Type:         "xfs",
										Mountpoint:   "/",
										FSTabOptions: "defaults",
									},
								},
							},
						},
					},
				},
			},
		},
		"btrfs": {
			customizations: &blueprint.PartitioningCustomization{
				Btrfs: &blueprint.BtrfsCustomization{
					Volumes: []blueprint.BtrfsVolumeCustomization{
						{
							MinSize: 230,
							Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
								{
									Name:       "subvol/root",
									Mountpoint: "/",
								},
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "subvol/varlog",
									Mountpoint: "/var/log",
								},
							},
						},
					},
				},
			},
			expected: &disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     512 * common.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    0,
						Size:     230,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						Payload: &disk.Btrfs{
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/root",
									Mountpoint: "/",
								},
								{
									Name:       "subvol/home",
									Mountpoint: "/home",
								},
								{
									Name:       "subvol/varlog",
									Mountpoint: "/var/log",
								},
							},
						},
					},
				},
			},
		},
		"the-works": { // this is silly
			customizations: &blueprint.PartitioningCustomization{
				Plain: &blueprint.PlainFilesystemCustomization{
					Mountpoints: []blueprint.MountpointCustomization{
						{
							Mountpoint: "/data",
							MinSize:    1000,
							Label:      "data",
							Type:       "ext4",
						},
						{
							Mountpoint: "/otherstuff",
							MinSize:    2000,
							Label:      "other-stuff",
							Type:       "ext4",
						},
					},
				},
				LVM: &blueprint.LVMCustomization{
					VolumeGroups: []blueprint.VGCustomization{
						{
							Name:    "testvg",
							MinSize: 199,
							LogicalVolumes: []blueprint.LVCustomization{
								{
									Name: "varloglv",
									MountpointCustomization: blueprint.MountpointCustomization{
										Mountpoint: "/var/log",
										MinSize:    50,
										Label:      "var-log",
										Type:       "xfs",
									},
								},
								{
									Name: "rootlv",
									MountpointCustomization: blueprint.MountpointCustomization{
										Mountpoint: "/",
										MinSize:    100,
										Label:      "root",
										Type:       "xfs",
									},
								},
							},
						},
					},
				},
				Btrfs: &blueprint.BtrfsCustomization{
					Volumes: []blueprint.BtrfsVolumeCustomization{
						{
							MinSize: 9999,
							Subvolumes: []blueprint.BtrfsSubvolumeCustomization{
								{
									Name:       "subvol/scratch",
									Mountpoint: "/scratch",
								},
							},
						},
					},
				},
			},
			expected: &disk.PartitionTable{
				Partitions: []disk.Partition{
					{
						Start:    0,
						Size:     512 * common.MiB,
						Type:     disk.XBootLDRPartitionGUID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Type:         "xfs",
							Label:        "boot",
							Mountpoint:   "/boot",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    0,
						Size:     1000,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Mountpoint:   "/data",
							Label:        "data",
							Type:         "ext4",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    0,
						Size:     2000,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						Payload: &disk.Filesystem{
							Mountpoint:   "/otherstuff",
							Label:        "other-stuff",
							Type:         "ext4",
							FSTabOptions: "defaults",
							FSTabFreq:    0,
							FSTabPassNo:  0,
						},
					},
					{
						Start:    0,
						Size:     199,
						Type:     disk.LVMPartitionGUID,
						Bootable: false,
						Payload: &disk.LVMVolumeGroup{
							Name:        "testvg",
							Description: "created via lvm2 and osbuild",
							LogicalVolumes: []disk.LVMLogicalVolume{
								{
									Name: "varloglv",
									Size: 50,
									Payload: &disk.Filesystem{
										Label:        "var-log",
										Mountpoint:   "/var/log",
										Type:         "xfs",
										FSTabOptions: "defaults",
										FSTabFreq:    0,
										FSTabPassNo:  0,
									},
								},
								{
									Name: "rootlv",
									Size: 100,
									Payload: &disk.Filesystem{
										Label:        "root",
										Mountpoint:   "/",
										Type:         "xfs",
										FSTabOptions: "defaults",
										FSTabFreq:    0,
										FSTabPassNo:  0,
									},
								},
							},
						},
					},
					{
						Start:    0,
						Size:     9999,
						Type:     disk.FilesystemDataGUID,
						Bootable: false,
						Payload: &disk.Btrfs{
							Subvolumes: []disk.BtrfsSubvolume{
								{
									Name:       "subvol/scratch",
									Mountpoint: "/scratch",
								},
							},
						},
					},
				},
			},
		},
	}

	for name := range testCases {
		tc := testCases[name]
		t.Run(name, func(t *testing.T) {
			pt, err := disk.NewCustomPartitionTable(tc.customizations, 0, nil, rnd)
			require.NoError(err)

			require.Equal(tc.expected, pt)
		})
	}

}
