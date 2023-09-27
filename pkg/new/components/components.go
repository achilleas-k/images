package components

import (
	"errors"
	"fmt"
	"math/rand"
	"sort"
	"strings"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/oscap"
	"github.com/osbuild/images/internal/pathpolicy"
	"github.com/osbuild/images/internal/users"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/image"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/runner"
	"golang.org/x/exp/slices"
)

const (
	osPkgsKey          = "os"
	blueprintPkgsKey   = "blueprint"
	cloudKernelOptions = "ro no_timer_check console=ttyS0,115200n8 biosdevname=0 net.ifnames=0"
)

var (
	oscapProfileAllowList = []oscap.Profile{
		oscap.Ospp,
		oscap.PciDss,
		oscap.Standard,
	}
)

type ImageConfig struct {
	platform    platform.Platform
	environment environment.Environment
	distro      distro.Distro
	workload    workload.Workload
}

func NewConfig() *ImageConfig {
	return &ImageConfig{}
}

func (ic *ImageConfig) SetPlatform(p platform.Platform) {
	ic.platform = p
}

func (ic *ImageConfig) SetEnvironment(env environment.Environment) {
	ic.environment = env
}

func (ic *ImageConfig) SetDistro(d distro.Distro) {
	ic.distro = d
}

func (ic *ImageConfig) SetWorkload(wl workload.Workload) {
	ic.workload = wl
}

func (ic *ImageConfig) ImageType() distro.ImageType {
	a := architecture{
		name:   ic.platform.GetArch().String(),
		distro: ic.distro,
	}
	it := &imageType{
		arch:        &a,
		platform:    ic.platform,
		environment: ic.environment,
		workload:    ic.workload,
		name:        "ami",
		nameAliases: []string{},
		filename:    "image.raw",
		compression: "",
		mimeType:    "application/octet-stream",
		packageSets: map[string]packageSetFunc{},
		defaultImageConfig: &distro.ImageConfig{
			DefaultTarget: common.ToPtr("multi-user.target"),
			EnabledServices: []string{
				"cloud-init.service",
				"cloud-config.service",
				"cloud-final.service",
				"cloud-init-local.service",
			},
		},
		kernelOptions:       cloudKernelOptions,
		defaultSize:         5 * common.GibiByte,
		buildPipelines:      []string{},
		payloadPipelines:    []string{},
		exports:             []string{},
		image:               diskImage,
		bootISO:             false,
		rpmOstree:           false,
		bootable:            true,
		basePartitionTables: defaultBasePartitionTables,
	}
	return it
}

// ====== IMAGE TYPE COPIED FROM FEDORA ===== //
type imageFunc func(workload workload.Workload, t *imageType, customizations *blueprint.Customizations, options distro.ImageOptions, packageSets map[string]rpmmd.PackageSet, containers []container.SourceSpec, rng *rand.Rand) (image.ImageKind, error)

type packageSetFunc func(t *imageType) rpmmd.PackageSet

type imageType struct {
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

	// bootISO: installable ISO
	bootISO bool
	// rpmOstree: iot/ostree
	rpmOstree bool
	// bootable image
	bootable bool
	// List of valid arches for the image type
	basePartitionTables    distro.BasePartitionTableMap
	requiredPartitionSizes map[string]uint64
}

func (t *imageType) Name() string {
	return t.name
}

func (t *imageType) Arch() distro.Arch {
	return t.arch
}

func (t *imageType) Filename() string {
	return t.filename
}

func (t *imageType) MIMEType() string {
	return t.mimeType
}

func (t *imageType) OSTreeRef() string {
	d := t.arch.distro
	if t.rpmOstree {
		return fmt.Sprintf(d.OSTreeRef(), t.arch.Name())
	}
	return ""
}

func (t *imageType) Size(size uint64) uint64 {
	// Microsoft Azure requires vhd images to be rounded up to the nearest MB
	if t.name == "vhd" && size%common.MebiByte != 0 {
		size = (size/common.MebiByte + 1) * common.MebiByte
	}
	if size == 0 {
		size = t.defaultSize
	}
	return size
}

func (t *imageType) BuildPipelines() []string {
	return t.buildPipelines
}

func (t *imageType) PayloadPipelines() []string {
	return t.payloadPipelines
}

func (t *imageType) PayloadPackageSets() []string {
	return []string{blueprintPkgsKey}
}

func (t *imageType) PackageSetsChains() map[string][]string {
	return make(map[string][]string)
}

func (t *imageType) Exports() []string {
	if len(t.exports) > 0 {
		return t.exports
	}
	return []string{"assembler"}
}

func (t *imageType) BootMode() distro.BootMode {
	if t.platform.GetUEFIVendor() != "" && t.platform.GetBIOSPlatform() != "" {
		return distro.BOOT_HYBRID
	} else if t.platform.GetUEFIVendor() != "" {
		return distro.BOOT_UEFI
	} else if t.platform.GetBIOSPlatform() != "" || t.platform.GetZiplSupport() {
		return distro.BOOT_LEGACY
	}
	return distro.BOOT_NONE
}

func (t *imageType) getPartitionTable(
	mountpoints []blueprint.FilesystemCustomization,
	options distro.ImageOptions,
	rng *rand.Rand,
) (*disk.PartitionTable, error) {
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
	if !exists {
		return nil, fmt.Errorf("unknown arch: " + t.arch.Name())
	}

	imageSize := t.Size(options.Size)

	lvmify := !t.rpmOstree

	return disk.NewPartitionTable(&basePartitionTable, mountpoints, imageSize, lvmify, t.requiredPartitionSizes, rng)
}

func (t *imageType) getDefaultImageConfig() *distro.ImageConfig {
	// ensure that image always returns non-nil default config
	imageConfig := t.defaultImageConfig
	if imageConfig == nil {
		imageConfig = &distro.ImageConfig{}
	}
	// return imageConfig.InheritFrom(t.arch.distro.getDefaultImageConfig())
	return imageConfig
}

func (t *imageType) PartitionType() string {
	basePartitionTable, exists := t.basePartitionTables[t.arch.Name()]
	if !exists {
		return ""
	}

	return basePartitionTable.Type
}

func (t *imageType) Manifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	seed int64) (*manifest.Manifest, []string, error) {

	warnings, err := t.checkOptions(bp, options)
	if err != nil {
		return nil, nil, err
	}

	// merge package sets that appear in the image type with the package sets
	// of the same name from the distro and arch
	staticPackageSets := make(map[string]rpmmd.PackageSet)

	for name, getter := range t.packageSets {
		staticPackageSets[name] = getter(t)
	}

	// amend with repository information and collect payload repos
	payloadRepos := make([]rpmmd.RepoConfig, 0)
	for _, repo := range repos {
		if len(repo.PackageSets) > 0 {
			// only apply the repo to the listed package sets
			for _, psName := range repo.PackageSets {
				if slices.Contains(t.PayloadPackageSets(), psName) {
					payloadRepos = append(payloadRepos, repo)
				}
				ps := staticPackageSets[psName]
				ps.Repositories = append(ps.Repositories, repo)
				staticPackageSets[psName] = ps
			}
		}
	}

	w := t.workload
	if w == nil {
		cw := &workload.Custom{
			BaseWorkload: workload.BaseWorkload{
				Repos: payloadRepos,
			},
			Packages: bp.GetPackagesEx(false),
		}
		if services := bp.Customizations.GetServices(); services != nil {
			cw.Services = services.Enabled
			cw.DisabledServices = services.Disabled
		}
		w = cw
	}

	containerSources := make([]container.SourceSpec, len(bp.Containers))
	for idx := range bp.Containers {
		containerSources[idx] = container.SourceSpec(bp.Containers[idx])
	}

	source := rand.NewSource(seed)
	// math/rand is good enough in this case
	/* #nosec G404 */
	rng := rand.New(source)

	img, err := t.image(w, t, bp.Customizations, options, staticPackageSets, containerSources, rng)
	if err != nil {
		return nil, nil, err
	}
	mf := manifest.New()
	mf.Distro = manifest.DISTRO_FEDORA

	// TODO: runner from distro
	_, err = img.InstantiateManifest(&mf, repos, &runner.Fedora{Version: 38}, rng)
	if err != nil {
		return nil, nil, err
	}

	return &mf, warnings, err
}

// checkOptions checks the validity and compatibility of options and customizations for the image type.
// Returns ([]string, error) where []string, if non-nil, will hold any generated warnings (e.g. deprecation notices).
func (t *imageType) checkOptions(bp *blueprint.Blueprint, options distro.ImageOptions) ([]string, error) {

	customizations := bp.Customizations

	// we do not support embedding containers on ostree-derived images, only on commits themselves
	if len(bp.Containers) > 0 && t.rpmOstree && (t.name != "iot-commit" && t.name != "iot-container") {
		return nil, fmt.Errorf("embedding containers is not supported for %s on %s", t.name, t.arch.distro.Name())
	}

	if options.OSTree != nil {
		if err := options.OSTree.Validate(); err != nil {
			return nil, err
		}
	}

	if t.bootISO && t.rpmOstree {
		// ostree-based ISOs require a URL from which to pull a payload commit
		if options.OSTree == nil || options.OSTree.URL == "" {
			return nil, fmt.Errorf("boot ISO image type %q requires specifying a URL from which to retrieve the OSTree commit", t.name)
		}
	}

	if t.name == "iot-raw-image" || t.name == "iot-qcow2-image" {
		allowed := []string{"User", "Group", "Directories", "Files", "Services"}
		if err := customizations.CheckAllowed(allowed...); err != nil {
			return nil, fmt.Errorf("unsupported blueprint customizations found for image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
		}
		// TODO: consider additional checks, such as those in "edge-simplified-installer" in RHEL distros
	}

	// BootISOs have limited support for customizations.
	// TODO: Support kernel name selection for image-installer
	if t.bootISO {
		if t.name == "iot-simplified-installer" {
			allowed := []string{"InstallationDevice", "FDO", "Ignition", "Kernel", "User", "Group"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return nil, fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
			if customizations.GetInstallationDevice() == "" {
				return nil, fmt.Errorf("boot ISO image type %q requires specifying an installation device to install to", t.name)
			}

			// FDO is optional, but when specified has some restrictions
			if customizations.GetFDO() != nil {
				if customizations.GetFDO().ManufacturingServerURL == "" {
					return nil, fmt.Errorf("boot ISO image type %q requires specifying FDO.ManufacturingServerURL configuration to install to when using FDO", t.name)
				}
				var diunSet int
				if customizations.GetFDO().DiunPubKeyHash != "" {
					diunSet++
				}
				if customizations.GetFDO().DiunPubKeyInsecure != "" {
					diunSet++
				}
				if customizations.GetFDO().DiunPubKeyRootCerts != "" {
					diunSet++
				}
				if diunSet != 1 {
					return nil, fmt.Errorf("boot ISO image type %q requires specifying one of [FDO.DiunPubKeyHash,FDO.DiunPubKeyInsecure,FDO.DiunPubKeyRootCerts] configuration to install to when using FDO", t.name)
				}
			}

			// ignition is optional, we might be using FDO
			if customizations.GetIgnition() != nil {
				if customizations.GetIgnition().Embedded != nil && customizations.GetIgnition().FirstBoot != nil {
					return nil, fmt.Errorf("both ignition embedded and firstboot configurations found")
				}
				if customizations.GetIgnition().FirstBoot != nil && customizations.GetIgnition().FirstBoot.ProvisioningURL == "" {
					return nil, fmt.Errorf("ignition.firstboot requires a provisioning url")
				}
			}
		} else if t.name == "iot-installer" || t.name == "image-installer" {
			allowed := []string{"User", "Group"}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return nil, fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: %s)", t.name, strings.Join(allowed, ", "))
			}
		} else if t.name == "live-installer" {
			allowed := []string{}
			if err := customizations.CheckAllowed(allowed...); err != nil {
				return nil, fmt.Errorf("unsupported blueprint customizations found for boot ISO image type %q: (allowed: None)", t.name)
			}
		}
	}

	if kernelOpts := customizations.GetKernel(); kernelOpts.Append != "" && t.rpmOstree {
		return nil, fmt.Errorf("kernel boot parameter customizations are not supported for ostree types")
	}

	mountpoints := customizations.GetFilesystems()

	if mountpoints != nil && t.rpmOstree {
		return nil, fmt.Errorf("Custom mountpoints are not supported for ostree types")
	}

	err := blueprint.CheckMountpointsPolicy(mountpoints, pathpolicy.MountpointPolicies)
	if err != nil {
		return nil, err
	}

	if osc := customizations.GetOpenSCAP(); osc != nil {
		supported := oscap.IsProfileAllowed(osc.ProfileID, oscapProfileAllowList)
		if !supported {
			return nil, fmt.Errorf(fmt.Sprintf("OpenSCAP unsupported profile: %s", osc.ProfileID))
		}
		if t.rpmOstree {
			return nil, fmt.Errorf("OpenSCAP customizations are not supported for ostree types")
		}
		if osc.ProfileID == "" {
			return nil, fmt.Errorf("OpenSCAP profile cannot be empty")
		}
	}

	// Check Directory/File Customizations are valid
	dc := customizations.GetDirectories()
	fc := customizations.GetFiles()

	err = blueprint.ValidateDirFileCustomizations(dc, fc)
	if err != nil {
		return nil, err
	}

	err = blueprint.CheckDirectoryCustomizationsPolicy(dc, pathpolicy.CustomDirectoriesPolicies)
	if err != nil {
		return nil, err
	}

	err = blueprint.CheckFileCustomizationsPolicy(fc, pathpolicy.CustomFilesPolicies)
	if err != nil {
		return nil, err
	}

	// check if repository customizations are valid
	_, err = customizations.GetRepositories()
	if err != nil {
		return nil, err
	}

	return nil, nil
}

type architecture struct {
	distro           distro.Distro
	name             string
	imageTypes       map[string]distro.ImageType
	imageTypeAliases map[string]string
}

func (a *architecture) Name() string {
	return a.name
}

func (a *architecture) ListImageTypes() []string {
	itNames := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		itNames = append(itNames, name)
	}
	sort.Strings(itNames)
	return itNames
}

func (a *architecture) GetImageType(name string) (distro.ImageType, error) {
	t, exists := a.imageTypes[name]
	if !exists {
		aliasForName, exists := a.imageTypeAliases[name]
		if !exists {
			return nil, errors.New("invalid image type: " + name)
		}
		t, exists = a.imageTypes[aliasForName]
		if !exists {
			panic(fmt.Sprintf("image type '%s' is an alias to a non-existing image type '%s'", name, aliasForName))
		}
	}
	return t, nil
}

func (a *architecture) Distro() distro.Distro {
	return a.distro
}

type distribution struct {
	name               string
	product            string
	osVersion          string
	releaseVersion     string
	modulePlatformID   string
	ostreeRefTmpl      string
	isolabelTmpl       string
	runner             runner.Runner
	arches             map[string]distro.Arch
	defaultImageConfig *distro.ImageConfig
}

func (d *distribution) Name() string {
	return d.name
}

func (d *distribution) Releasever() string {
	return d.releaseVersion
}

func (d *distribution) ModulePlatformID() string {
	return d.modulePlatformID
}

func (d *distribution) OSTreeRef() string {
	return d.ostreeRefTmpl
}

func (d *distribution) ListArches() []string {
	archNames := make([]string, 0, len(d.arches))
	for name := range d.arches {
		archNames = append(archNames, name)
	}
	sort.Strings(archNames)
	return archNames
}

func (d *distribution) GetArch(name string) (distro.Arch, error) {
	arch, exists := d.arches[name]
	if !exists {
		return nil, errors.New("invalid architecture: " + name)
	}
	return arch, nil
}

func (d *distribution) addArches(arches ...architecture) {
	if d.arches == nil {
		d.arches = map[string]distro.Arch{}
	}

	// Do not make copies of architectures, as opposed to image types,
	// because architecture definitions are not used by more than a single
	// distro definition.
	for idx := range arches {
		d.arches[arches[idx].name] = &arches[idx]
	}
}

func (d *distribution) getDefaultImageConfig() *distro.ImageConfig {
	return d.defaultImageConfig
}

var defaultBasePartitionTables = distro.BasePartitionTableMap{
	platform.ARCH_X86_64.String(): disk.PartitionTable{
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
	},
	platform.ARCH_AARCH64.String(): disk.PartitionTable{
		UUID: "D209C89E-EA5E-4FBD-B161-B461CCE297E0",
		Type: "gpt",
		Partitions: []disk.Partition{
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
	},
}

func osCustomizations(
	t *imageType,
	osPackageSet rpmmd.PackageSet,
	containers []container.SourceSpec,
	c *blueprint.Customizations) manifest.OSCustomizations {

	imageConfig := t.getDefaultImageConfig()

	osc := manifest.OSCustomizations{}

	if t.bootable || t.rpmOstree {
		osc.KernelName = c.GetKernel().Name

		var kernelOptions []string
		if t.kernelOptions != "" {
			kernelOptions = append(kernelOptions, t.kernelOptions)
		}
		if bpKernel := c.GetKernel(); bpKernel.Append != "" {
			kernelOptions = append(kernelOptions, bpKernel.Append)
		}
		osc.KernelOptionsAppend = kernelOptions
	}

	osc.ExtraBasePackages = osPackageSet.Include
	osc.ExcludeBasePackages = osPackageSet.Exclude
	osc.ExtraBaseRepos = osPackageSet.Repositories

	osc.Containers = containers

	osc.GPGKeyFiles = imageConfig.GPGKeyFiles
	if imageConfig.ExcludeDocs != nil {
		osc.ExcludeDocs = *imageConfig.ExcludeDocs
	}

	if !t.bootISO {
		// don't put users and groups in the payload of an installer
		// add them via kickstart instead
		osc.Groups = users.GroupsFromBP(c.GetGroups())
		osc.Users = users.UsersFromBP(c.GetUsers())
	}

	osc.EnabledServices = imageConfig.EnabledServices
	osc.DisabledServices = imageConfig.DisabledServices
	if imageConfig.DefaultTarget != nil {
		osc.DefaultTarget = *imageConfig.DefaultTarget
	}

	if fw := c.GetFirewall(); fw != nil {
		options := osbuild.FirewallStageOptions{
			Ports: fw.Ports,
		}

		if fw.Services != nil {
			options.EnabledServices = fw.Services.Enabled
			options.DisabledServices = fw.Services.Disabled
		}
		osc.Firewall = &options
	}

	language, keyboard := c.GetPrimaryLocale()
	if language != nil {
		osc.Language = *language
	} else if imageConfig.Locale != nil {
		osc.Language = *imageConfig.Locale
	}
	if keyboard != nil {
		osc.Keyboard = keyboard
	} else if imageConfig.Keyboard != nil {
		osc.Keyboard = &imageConfig.Keyboard.Keymap
	}

	if hostname := c.GetHostname(); hostname != nil {
		osc.Hostname = *hostname
	} else {
		osc.Hostname = "localhost.localdomain"
	}

	timezone, ntpServers := c.GetTimezoneSettings()
	if timezone != nil {
		osc.Timezone = *timezone
	} else if imageConfig.Timezone != nil {
		osc.Timezone = *imageConfig.Timezone
	}

	if len(ntpServers) > 0 {
		for _, server := range ntpServers {
			osc.NTPServers = append(osc.NTPServers, osbuild.ChronyConfigServer{Hostname: server})
		}
	} else if imageConfig.TimeSynchronization != nil {
		osc.NTPServers = imageConfig.TimeSynchronization.Servers
	}

	// Relabel the tree, unless the `NoSElinux` flag is explicitly set to `true`
	if imageConfig.NoSElinux == nil || imageConfig.NoSElinux != nil && !*imageConfig.NoSElinux {
		osc.SElinux = "targeted"
	}

	var err error
	osc.Directories, err = blueprint.DirectoryCustomizationsToFsNodeDirectories(c.GetDirectories())
	if err != nil {
		// In theory this should never happen, because the blueprint directory customizations
		// should have been validated before this point.
		panic(fmt.Sprintf("failed to convert directory customizations to fs node directories: %v", err))
	}

	osc.Files, err = blueprint.FileCustomizationsToFsNodeFiles(c.GetFiles())
	if err != nil {
		// In theory this should never happen, because the blueprint file customizations
		// should have been validated before this point.
		panic(fmt.Sprintf("failed to convert file customizations to fs node files: %v", err))
	}

	customRepos, err := c.GetRepositories()
	if err != nil {
		// This shouldn't happen and since the repos
		// should have already been validated
		panic(fmt.Sprintf("failed to get custom repos: %v", err))
	}

	// This function returns a map of filename and corresponding yum repos
	// and a list of fs node files for the inline gpg keys so we can save
	// them to disk. This step also swaps the inline gpg key with the path
	// to the file in the os file tree
	yumRepos, gpgKeyFiles, err := blueprint.RepoCustomizationsToRepoConfigAndGPGKeyFiles(customRepos)
	if err != nil {
		panic(fmt.Sprintf("failed to convert inline gpgkeys to fs node files: %v", err))
	}

	// add the gpg key files to the list of files to be added to the tree
	if len(gpgKeyFiles) > 0 {
		osc.Files = append(osc.Files, gpgKeyFiles...)
	}

	for filename, repos := range yumRepos {
		osc.YUMRepos = append(osc.YUMRepos, osbuild.NewYumReposStageOptions(filename, repos))
	}

	if oscapConfig := c.GetOpenSCAP(); oscapConfig != nil {
		if t.rpmOstree {
			panic("unexpected oscap options for ostree image type")
		}
		var datastream = oscapConfig.DataStream
		if datastream == "" {
			datastream = oscap.DefaultFedoraDatastream()
		}

		oscapStageOptions := osbuild.OscapConfig{
			Datastream: datastream,
			ProfileID:  oscapConfig.ProfileID,
		}

		if oscapConfig.Tailoring != nil {
			newProfile, tailoringFilepath, tailoringDir, err := oscap.GetTailoringFile(oscapConfig.ProfileID)
			if err != nil {
				panic(fmt.Sprintf("unexpected error creating tailoring file options: %v", err))
			}

			tailoringOptions := osbuild.OscapAutotailorConfig{
				Selected:   oscapConfig.Tailoring.Selected,
				Unselected: oscapConfig.Tailoring.Unselected,
				NewProfile: newProfile,
			}

			osc.OpenSCAPTailorConfig = osbuild.NewOscapAutotailorStageOptions(
				tailoringFilepath,
				oscapStageOptions,
				tailoringOptions,
			)

			// overwrite the profile id with the new tailoring id
			oscapStageOptions.ProfileID = newProfile
			oscapStageOptions.Tailoring = tailoringFilepath

			// add the parent directory for the tailoring file
			osc.Directories = append(osc.Directories, tailoringDir)
		}

		osc.OpenSCAPConfig = osbuild.NewOscapRemediationStageOptions(oscapStageOptions)
	}

	osc.ShellInit = imageConfig.ShellInit

	osc.Grub2Config = imageConfig.Grub2Config
	osc.Sysconfig = imageConfig.Sysconfig
	osc.SystemdLogind = imageConfig.SystemdLogind
	osc.CloudInit = imageConfig.CloudInit
	osc.Modprobe = imageConfig.Modprobe
	osc.DracutConf = imageConfig.DracutConf
	osc.SystemdUnit = imageConfig.SystemdUnit
	osc.Authselect = imageConfig.Authselect
	osc.SELinuxConfig = imageConfig.SELinuxConfig
	osc.Tuned = imageConfig.Tuned
	osc.Tmpfilesd = imageConfig.Tmpfilesd
	osc.PamLimitsConf = imageConfig.PamLimitsConf
	osc.Sysctld = imageConfig.Sysctld
	osc.DNFConfig = imageConfig.DNFConfig
	osc.SshdConfig = imageConfig.SshdConfig
	osc.AuthConfig = imageConfig.Authconfig
	osc.PwQuality = imageConfig.PwQuality
	osc.WSLConfig = imageConfig.WSLConfig

	osc.Files = append(osc.Files, imageConfig.Files...)
	osc.Directories = append(osc.Directories, imageConfig.Directories...)

	return osc
}

// IMAGES

func diskImage(workload workload.Workload,
	t *imageType,
	customizations *blueprint.Customizations,
	options distro.ImageOptions,
	packageSets map[string]rpmmd.PackageSet,
	containers []container.SourceSpec,
	rng *rand.Rand) (image.ImageKind, error) {

	img := image.NewDiskImage()
	img.Platform = t.platform
	img.OSCustomizations = osCustomizations(t, packageSets[osPkgsKey], containers, customizations)
	img.Environment = t.environment
	img.Workload = workload
	img.Compression = t.compression
	// TODO: move generation into LiveImage
	pt, err := t.getPartitionTable(customizations.GetFilesystems(), options, rng)
	if err != nil {
		return nil, err
	}
	img.PartitionTable = pt

	img.Filename = t.Filename()

	return img, nil
}
