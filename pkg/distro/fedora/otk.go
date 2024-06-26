package fedora

import (
	"fmt"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

type otkImageType struct {
	arch               *architecture
	platform           platform.Platform
	environment        environment.Environment
	workload           workload.Workload
	name               string
	nameAliases        []string
	filename           string
	compression        string
	mimeType           string
	packageSets        map[string]packageSetFunc
	defaultImageConfig *distro.ImageConfig
	kernelOptions      string
	defaultSize        uint64
	buildPipelines     []string
	payloadPipelines   []string
	exports            []string
	image              imageFunc
	isoLabel           isoLabelFunc

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: iot/ostree
	rpmOstree bool
	// bootable image
	bootable bool
	// List of valid arches for the image type
	basePartitionTables    distro.BasePartitionTableMap
	requiredPartitionSizes map[string]uint64

	otkPath string
}

func NewOtkImageType(path string) otkImageType {
	baseFilename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	it := otkImageType{
		name:     "otk." + baseFilename,
		filename: "disk.qcow2",
		otkPath:  path,
	}
	return it
}

func (t *otkImageType) Name() string {
	return t.name
}

func (t *otkImageType) Arch() distro.Arch {
	return t.arch
}

func (t *otkImageType) Filename() string {
	return t.filename
}

func (t *otkImageType) MIMEType() string {
	return t.mimeType
}

func (t *otkImageType) OSTreeRef() string {
	d := t.arch.distro
	if t.rpmOstree {
		return fmt.Sprintf(d.ostreeRefTmpl, t.arch.Name())
	}
	return ""
}

func (t *otkImageType) ISOLabel() (string, error) {
	if !t.bootISO {
		return "", fmt.Errorf("image type %q is not an ISO", t.name)
	}

	// if t.isoLabel != nil {
	// 	return t.isoLabel(t), nil
	// }

	return "", nil
}

func (t *otkImageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%common.MebiByte != 0 {
		size = (size/common.MebiByte + 1) * common.MebiByte
	}
	if size == 0 {
		size = t.defaultSize
	}
	return size
}

func (t *otkImageType) BuildPipelines() []string {
	return t.buildPipelines
}

func (t *otkImageType) PayloadPipelines() []string {
	return t.payloadPipelines
}

func (t *otkImageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *otkImageType) PackageSetsChains() map[string][]string {
	return make(map[string][]string)
}

func (t *otkImageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

func (t *otkImageType) BootMode() distro.BootMode {
	if t.platform.GetUEFIVendor() != "" && t.platform.GetBIOSPlatform() != "" {
		return distro.BOOT_HYBRID
	} else if t.platform.GetUEFIVendor() != "" {
		return distro.BOOT_UEFI
	} else if t.platform.GetBIOSPlatform() != "" || t.platform.GetZiplSupport() {
		return distro.BOOT_LEGACY
	}
	return distro.BOOT_NONE
}

func (t *otkImageType) PartitionType() string {
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

func (t *otkImageType) Manifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	seed int64) (*manifest.Manifest, []string, error) {

	mf := manifest.New()
	mf.OTKPath = t.otkPath
	return &mf, nil, nil
}
