package blueprint

import (
	"encoding/json"
	"errors"
	"fmt"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/pathpolicy"
)

type FilesystemCustomization MountpointCustomization

type MountpointCustomization struct {
	Mountpoint string `json:"mountpoint" toml:"mountpoint"`
	MinSize    uint64 `json:"minsize,omitempty" toml:"minsize,omitempty"`
	Label      string `json:"label,omitempty" toml:"label,omitempty"`
	Type       string `json:"type,omitempty" toml:"type,omitempty"`
}

type PlainFilesystemCustomization struct {
	Mountpoints []MountpointCustomization `json:"mountpoints,omitempty" toml:"mountpoints,omitempty"`
}

type LVCustomization struct {
	// Logical volume name
	Name string
	MountpointCustomization
}

type VGCustomization struct {
	// Volume group name
	Name string `json:"name" toml:"name"`
	// Size of the partition that contains the volume group
	Size           string            `json:"size" toml:"size"`
	LogicalVolumes []LVCustomization `json:"mountpoints,omitempty" toml:"mountpoints,omitempty"`
}

type LVMCustomization struct {
	VolumeGroups []VGCustomization `json:"volume-groups,omitempty" toml:"volume-groups,omitempty"`
}

type BtrfsSubvolumeCustomization struct {
	Mountpoint string `json:"mountpoint" toml:"mountpoint"`
	// Qgroup in the future??
}

type BtrfsVolumeCustomization struct {
	// Size of the btrfs partition/volume.
	Size       uint64 `json:"size" toml:"size"`
	Subvolumes []BtrfsSubvolumeCustomization
}

type BtrfsCustomization struct {
	Volumes []BtrfsVolumeCustomization
}

type LuksParameterCustomization struct {
	// Iterations and stuff?
}

type LUKSCustomization struct {
	Params LuksParameterCustomization    `json:"params" toml:"params"`
	Plain  *PlainFilesystemCustomization `json:"plain,omitempty" toml:"plain,omitempty"`
	LVM    *LVMCustomization             `json:"lvm,omitempty" toml:"lvm,omitempty"`
	Btrfs  *BtrfsCustomization           `json:"btrfs,omitempty" toml:"btrfs,omitempty"`
}

type PartitioningCustomization struct {
	Plain   *PlainFilesystemCustomization `json:"plain,omitempty" toml:"plain,omitempty"`
	LVM     *LVMCustomization             `json:"lvm,omitempty" toml:"lvm,omitempty"`
	Btrfs   *BtrfsCustomization           `json:"btrfs,omitempty" toml:"btrfs,omitempty"`
	LUKS    bool                          `json:"luks" toml:"luks"`
	LUKSAlt *LUKSCustomization            `json:"luksalt" toml:"luksalt"` // alternative to the boolean above
}

func (fsc *FilesystemCustomization) UnmarshalTOML(data interface{}) error {
	d, _ := data.(map[string]interface{})

	switch d["mountpoint"].(type) {
	case string:
		fsc.Mountpoint = d["mountpoint"].(string)
	default:
		return fmt.Errorf("TOML unmarshal: mountpoint must be string, got %v of type %T", d["mountpoint"], d["mountpoint"])
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
