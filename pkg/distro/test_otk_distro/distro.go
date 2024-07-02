package test_otk_distro

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/rpmmd"
)

type Distro struct {
	name    string
	arches  map[string]distro.Arch
	otkRoot string
}

func New(otkRoot string) distro.Distro {
	// use path basename as distro name
	name := filepath.Base(otkRoot)

	d := &Distro{
		name:    name,
		otkRoot: otkRoot,
	}
	d.readArches()
	return d
}

func (d *Distro) readArches() {
	// read otkRoot and discover arches based on expected directory structure
	entries, err := os.ReadDir(d.otkRoot)
	if err != nil {
		panic(fmt.Sprintf("failed to read otk root for test distro: %s", err))
	}

	d.arches = make(map[string]distro.Arch)
	for _, entry := range entries {
		if !entry.IsDir() {
			// ignore files
			continue
		}
		// assume each subdir is an architecture
		name := entry.Name()
		a := Arch{
			name:    name,
			otkRoot: filepath.Join(d.otkRoot, entry.Name()),
		}
		a.readImageTypes()
		d.arches[a.Name()] = a
	}
}

func (d Distro) Name() string {
	return d.name
}

func (d Distro) Codename() string {
	return "otd"
}

func (d Distro) Releasever() string {
	return "42"
}

func (d Distro) OsVersion() string {
	return "42"
}

func (d Distro) ModulePlatformID() string {
	return "platform:otk"
}

func (d Distro) Product() string {
	return "otk-test-distro"
}

func (d Distro) OSTreeRef() string {
	return ""
}

func (d Distro) ListArches() []string {
	arches := make([]string, 0, len(d.arches))
	for arch := range d.arches {
		arches = append(arches, arch)
	}
	return arches
}

func (d Distro) GetArch(name string) (distro.Arch, error) {
	return d.arches[name], nil
}

type Arch struct {
	distribution *Distro
	name         string
	imageTypes   map[string]distro.ImageType
	otkRoot      string
}

func (a *Arch) readImageTypes() {
	entries, err := os.ReadDir(a.otkRoot)
	if err != nil {
		panic(fmt.Sprintf("error reading architecture directory %q for otk test distro: %s", a.otkRoot, err))
	}

	a.imageTypes = make(map[string]distro.ImageType)
	for _, entry := range entries {
		if entry.IsDir() {
			// ignore subdirectories
			continue
		}

		switch filepath.Ext(entry.Name()) {
		case ".yml", ".yaml":
			imageType := newOtkImageType(filepath.Join(a.otkRoot, entry.Name()))
			a.imageTypes[imageType.Name()] = &imageType
		}
	}
}

func (a Arch) Name() string {
	return a.name
}

func (a Arch) Distro() distro.Distro {
	return a.distribution
}

func (a Arch) ListImageTypes() []string {
	names := make([]string, 0, len(a.imageTypes))
	for name := range a.imageTypes {
		names = append(names, name)
	}
	return names
}

func (a Arch) GetImageType(name string) (distro.ImageType, error) {
	return a.imageTypes[name], nil
}

type otkImageType struct {
	arch    *Arch
	name    string
	otkPath string
}

func newOtkImageType(path string) otkImageType {
	baseFilename := strings.TrimSuffix(filepath.Base(path), filepath.Ext(path))
	it := otkImageType{
		name:    baseFilename,
		otkPath: path,
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
	return ""
}

func (t *otkImageType) MIMEType() string {
	return ""
}

func (t *otkImageType) OSTreeRef() string {
	return ""
}

func (t *otkImageType) ISOLabel() (string, error) {
	return "", nil
}

func (t *otkImageType) Size(size uint64) uint64 {
	return size
}

func (t *otkImageType) BuildPipelines() []string {
	return nil
}

func (t *otkImageType) PayloadPipelines() []string {
	return nil
}

func (t *otkImageType) PayloadPackageSets() []string {
	return nil
}

func (t *otkImageType) PackageSetsChains() map[string][]string {
	return nil
}

func (t *otkImageType) Exports() []string {
	return nil
}

func (t *otkImageType) BootMode() distro.BootMode {
	return distro.BOOT_NONE
}

func (t *otkImageType) PartitionType() string {
	return ""
}

func (t *otkImageType) Manifest(bp *blueprint.Blueprint,
	options distro.ImageOptions,
	repos []rpmmd.RepoConfig,
	seed int64) (manifest.Manifest, []string, error) {

	mf := manifest.NewOTK(t.otkPath)
	return mf, nil, nil
}
