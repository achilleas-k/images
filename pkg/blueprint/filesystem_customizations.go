package blueprint

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/pathpolicy"
)

// TODO: validate input:
// - Duplicate mountpoints
// - No mixing of btrfs and LVM
// - Only one swap partition or file

type PartitioningCustomization struct {
	MinSize uint64                        `json:"minsize,omitempty" toml:"minsize,omitempty"`
	Plain   *PlainFilesystemCustomization `json:"plain,omitempty" toml:"plain,omitempty"`
	LVM     *LVMCustomization             `json:"lvm,omitempty" toml:"lvm,omitempty"`
	Btrfs   *BtrfsCustomization           `json:"btrfs,omitempty" toml:"btrfs,omitempty"`
}

type PlainFilesystemCustomization struct {
	Filesystems []FilesystemCustomization `json:"filesystems,omitempty" toml:"filesystems,omitempty"`
}

type FilesystemCustomization struct {
	Mountpoint string `json:"mountpoint" toml:"mountpoint"`
	MinSize    uint64 `json:"minsize,omitempty" toml:"minsize,omitempty"`
	Label      string `json:"label,omitempty" toml:"label,omitempty"`
	Type       string `json:"type,omitempty" toml:"type,omitempty"`
}

type LVMCustomization struct {
	VolumeGroups []VGCustomization `json:"volume_groups,omitempty" toml:"volume_groups,omitempty"`
}

type VGCustomization struct {
	// Volume group name
	Name string `json:"name" toml:"name"`
	// Size of the partition that contains the volume group
	MinSize        uint64            `json:"minsize" toml:"minsize"`
	LogicalVolumes []LVCustomization `json:"logical_volumes,omitempty" toml:"logical_volumes,omitempty"`
}

type LVCustomization struct {
	// Logical volume name
	Name string `json:"name,omitempty" toml:"name,omitempty"`
	FilesystemCustomization
}

type BtrfsCustomization struct {
	Volumes []BtrfsVolumeCustomization
}

type BtrfsVolumeCustomization struct {
	// Size of the btrfs partition/volume.
	MinSize    uint64 `json:"minsize" toml:"minsize"`
	Subvolumes []BtrfsSubvolumeCustomization
}

type BtrfsSubvolumeCustomization struct {
	Name       string `json:"name" toml:"name"`
	Mountpoint string `json:"mountpoint" toml:"mountpoint"`
}

func (fsc *FilesystemCustomization) UnmarshalTOML(data interface{}) error {
	d, _ := data.(map[string]interface{})

	switch d["mountpoint"].(type) {
	case string:
		fsc.Mountpoint = d["mountpoint"].(string)
	default:
		return fmt.Errorf("TOML unmarshal: mountpoint must be string, got %v of type %T", d["mountpoint"], d["mountpoint"])
	}

	switch d["type"].(type) {
	case nil:
		// empty allowed
	case string:
		fsc.Type = d["type"].(string)
	default:
		return fmt.Errorf("TOML unmarshal: type must be string, got %v of type %T", d["type"], d["type"])
	}

	switch d["label"].(type) {
	case nil:
		// empty allowed
	case string:
		fsc.Label = d["label"].(string)
	default:
		return fmt.Errorf("TOML unmarshal: label must be string, got %v of type %T", d["label"], d["label"])
	}

	switch d["minsize"].(type) {
	case int64:
		fsc.MinSize = uint64(d["minsize"].(int64))
	case string:
		minSize, err := common.DataSizeToUint64(d["minsize"].(string))
		if err != nil {
			return fmt.Errorf("TOML unmarshal: minsize is not valid filesystem size (%w)", err)
		}
		fsc.MinSize = minSize
	default:
		return fmt.Errorf("TOML unmarshal: minsize must be integer or string, got %v of type %T", d["minsize"], d["minsize"])
	}

	return nil
}

func (fsc *FilesystemCustomization) UnmarshalJSON(data []byte) error {
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return err
	}
	d, _ := v.(map[string]interface{})

	switch d["mountpoint"].(type) {
	case string:
		fsc.Mountpoint = d["mountpoint"].(string)
	default:
		return fmt.Errorf("JSON unmarshal: mountpoint must be string, got %v of type %T", d["mountpoint"], d["mountpoint"])
	}

	switch d["type"].(type) {
	case nil:
		// empty allowed
	case string:
		fsc.Type = d["type"].(string)
	default:
		return fmt.Errorf("JSON unmarshal: type must be string, got %v of type %T", d["type"], d["type"])
	}

	switch d["label"].(type) {
	case nil:
		// empty allowed
	case string:
		fsc.Label = d["label"].(string)
	default:
		return fmt.Errorf("JSON unmarshal: label must be string, got %v of type %T", d["label"], d["label"])
	}

	// The JSON specification only mentions float64 and Go defaults to it: https://go.dev/blog/json
	switch d["minsize"].(type) {
	case float64:
		fsc.MinSize = uint64(d["minsize"].(float64))
	case string:
		minSize, err := common.DataSizeToUint64(d["minsize"].(string))
		if err != nil {
			return fmt.Errorf("JSON unmarshal: minsize is not valid filesystem size (%w)", err)
		}
		fsc.MinSize = minSize
	default:
		return fmt.Errorf("JSON unmarshal: minsize must be float64 number or string, got %v of type %T", d["minsize"], d["minsize"])
	}

	return nil
}

// CheckMountpointsPolicy checks if the mountpoints are allowed by the policy
func CheckMountpointsPolicy(mountpoints []FilesystemCustomization, mountpointAllowList *pathpolicy.PathPolicies) error {
	var errs []error
	for _, m := range mountpoints {
		if err := mountpointAllowList.Check(m.Mountpoint); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("The following errors occurred while setting up custom mountpoints:\n%w", errors.Join(errs...))
	}

	return nil
}

// CheckMountpointsPolicy checks if the mountpoints are allowed by the policy
func CheckPartitioningPolicy(partitioning *PartitioningCustomization, mountpointAllowList *pathpolicy.PathPolicies) error {
	if partitioning == nil {
		return nil
	}

	// collect all mountpoints
	mountpoints := make([]string, 0)
	if partitioning.Plain != nil {
		for _, part := range partitioning.Plain.Filesystems {
			mountpoints = append(mountpoints, part.Mountpoint)
		}
	}
	if partitioning.LVM != nil {
		for _, vg := range partitioning.LVM.VolumeGroups {
			for _, lv := range vg.LogicalVolumes {
				mountpoints = append(mountpoints, lv.Mountpoint)
			}
		}
	}
	if partitioning.Btrfs != nil {
		for _, vol := range partitioning.Btrfs.Volumes {
			for _, subvol := range vol.Subvolumes {
				mountpoints = append(mountpoints, subvol.Mountpoint)
			}
		}
	}

	var errs []error
	for _, mp := range mountpoints {
		if err := mountpointAllowList.Check(mp); err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) > 0 {
		return fmt.Errorf("The following errors occurred while setting up custom mountpoints:\n%w", errors.Join(errs...))
	}

	return nil
}
