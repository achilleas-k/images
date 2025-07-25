package generic_test

import (
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/pkg/arch"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/distro_test_common"
	"github.com/osbuild/images/pkg/distro/generic"
)

type rhel10FamilyDistro struct {
	name   string
	distro distro.Distro
}

var rhel10FamilyDistros = []rhel10FamilyDistro{
	{
		name:   "rhel-10.0",
		distro: generic.DistroFactory("rhel-10.0"),
	},
}

func TestRH10FilenameFromType(t *testing.T) {
	type args struct {
		outputFormat string
	}
	type wantResult struct {
		filename string
		mimeType string
		wantErr  bool
	}
	tests := []struct {
		name string
		args args
		want wantResult
	}{
		{
			name: "ami",
			args: args{"ami"},
			want: wantResult{
				filename: "image.raw",
				mimeType: "application/octet-stream",
			},
		},
		{
			name: "qcow2",
			args: args{"qcow2"},
			want: wantResult{
				filename: "disk.qcow2",
				mimeType: "application/x-qemu-disk",
			},
		},
		{
			name: "vhd",
			args: args{"vhd"},
			want: wantResult{
				filename: "disk.vhd",
				mimeType: "application/x-vhd",
			},
		},
		{
			name: "vmdk",
			args: args{"vmdk"},
			want: wantResult{
				filename: "disk.vmdk",
				mimeType: "application/x-vmdk",
			},
		},
		{
			name: "ova",
			args: args{"ova"},
			want: wantResult{
				filename: "image.ova",
				mimeType: "application/ovf",
			},
		},
		{
			name: "tar",
			args: args{"tar"},
			want: wantResult{
				filename: "root.tar.xz",
				mimeType: "application/x-tar",
			},
		},
		{
			name: "invalid-output-type",
			args: args{"foobar"},
			want: wantResult{wantErr: true},
		},
	}
	for _, dist := range rhel10FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, tt := range tests {
				t.Run(tt.name, func(t *testing.T) {
					dist := dist.distro
					arch, _ := dist.GetArch("x86_64")
					imgType, err := arch.GetImageType(tt.args.outputFormat)
					if tt.want.wantErr {
						require.Error(t, err)
					} else {
						require.NoError(t, err)
						require.NotNil(t, imgType)
						gotFilename := imgType.Filename()
						gotMIMEType := imgType.MIMEType()
						if gotFilename != tt.want.filename {
							t.Errorf("ImageType.Filename()  got = %v, want %v", gotFilename, tt.want.filename)
						}
						if gotMIMEType != tt.want.mimeType {
							t.Errorf("ImageType.MIMEType() got1 = %v, want %v", gotMIMEType, tt.want.mimeType)
						}
					}
				})
			}
		})
	}
}

func TestRH10ImageType_BuildPackages(t *testing.T) {
	x8664BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"grub2-efi-x64",
		"grub2-pc",
		"policycoreutils",
		"shim-x64",
		"systemd",
		"tar",
		"qemu-img",
		"xz",
	}
	aarch64BuildPackages := []string{
		"dnf",
		"dosfstools",
		"e2fsprogs",
		"policycoreutils",
		"qemu-img",
		"systemd",
		"tar",
		"xz",
	}
	buildPackages := map[string][]string{
		"x86_64":  x8664BuildPackages,
		"aarch64": aarch64BuildPackages,
	}
	for _, dist := range rhel10FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			d := dist.distro
			for _, archLabel := range d.ListArches() {
				archStruct, err := d.GetArch(archLabel)
				if assert.NoErrorf(t, err, "d.GetArch(%v) returned err = %v; expected nil", archLabel, err) {
					continue
				}
				for _, itLabel := range archStruct.ListImageTypes() {
					itStruct, err := archStruct.GetImageType(itLabel)
					if assert.NoErrorf(t, err, "d.GetArch(%v) returned err = %v; expected nil", archLabel, err) {
						continue
					}
					manifest, _, err := itStruct.Manifest(&blueprint.Blueprint{}, distro.ImageOptions{}, nil, nil)
					assert.NoError(t, err)
					buildPkgs := manifest.GetPackageSetChains()["build"]
					assert.NotNil(t, buildPkgs)
					assert.Len(t, buildPkgs, 1)
					assert.ElementsMatch(t, buildPackages[archLabel], buildPkgs[0].Include)
				}
			}
		})
	}
}

func TestRH10ImageType_Name(t *testing.T) {
	imgMap := []struct {
		arch     string
		imgNames []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"qcow2",
				"vhd",
				"vmdk",
				"ova",
				"ami",
				"tar",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"qcow2",
				"ami",
				"tar",
				"vhd",
			},
		},
		{
			arch: "ppc64le",
			imgNames: []string{
				"qcow2",
				"tar",
			},
		},
		{
			arch: "s390x",
			imgNames: []string{
				"qcow2",
				"tar",
			},
		},
	}

	for _, dist := range rhel10FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				if mapping.arch == arch.ARCH_S390X.String() && dist.name == "centos" {
					continue
				}
				arch, err := dist.distro.GetArch(mapping.arch)
				if assert.NoError(t, err) {
					for _, imgName := range mapping.imgNames {
						imgType, err := arch.GetImageType(imgName)
						if assert.NoError(t, err) {
							assert.Equalf(t, imgName, imgType.Name(), "arch: %s", mapping.arch)
						}
					}
				}
			}
		})
	}
}

// Check that Manifest() function returns an error for unsupported
// configurations.
func TestRH10Distro_ManifestError(t *testing.T) {
	// Currently, the only unsupported configuration is OSTree commit types
	// with Kernel boot options
	r10distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Kernel: &blueprint.KernelCustomization{
				Append: "debug",
			},
		},
	}

	for _, archName := range r10distro.ListArches() {
		arch, _ := r10distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			imgOpts := distro.ImageOptions{
				Size: imgType.Size(0),
			}
			_, _, err := imgType.Manifest(&bp, imgOpts, nil, nil)
			assert.NoError(t, err)
		}
	}
}

func TestRH10Architecture_ListImageTypes(t *testing.T) {
	imgMap := []struct {
		arch                     string
		imgNames                 []string
		rhelAdditionalImageTypes []string
	}{
		{
			arch: "x86_64",
			imgNames: []string{
				"qcow2",
				"oci",
				"vhd",
				"vmdk",
				"ova",
				"ami",
				"tar",
				"wsl",
				"gce",
				"image-installer",
				"azure-cvm",
				"azure-rhui",
				"azure-sap-rhui",
				"azure-sapapps-rhui",
				"ec2",
				"ec2-ha",
				"ec2-sap",
				"vagrant-libvirt",
				"vagrant-virtualbox",
			},
		},
		{
			arch: "aarch64",
			imgNames: []string{
				"ami",
				"azure-rhui",
				"ec2",
				"image-installer",
				"qcow2",
				"tar",
				"vagrant-libvirt",
				"vhd",
				"wsl",
			},
		},
		{
			arch: "ppc64le",
			imgNames: []string{
				"qcow2",
				"tar",
			},
		},
		{
			arch: "s390x",
			imgNames: []string{
				"qcow2",
				"tar",
			},
		},
	}

	for _, dist := range rhel10FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, mapping := range imgMap {
				arch, err := dist.distro.GetArch(mapping.arch)
				require.NoError(t, err)
				imageTypes := arch.ListImageTypes()

				var expectedImageTypes []string
				expectedImageTypes = append(expectedImageTypes, mapping.imgNames...)
				if dist.name == "rhel" {
					expectedImageTypes = append(expectedImageTypes, mapping.rhelAdditionalImageTypes...)
				}

				sort.Strings(expectedImageTypes)
				sort.Strings(imageTypes)
				require.Equal(t, expectedImageTypes, imageTypes)
			}
		})
	}
}

func TestRH10Rhel10_ListArches(t *testing.T) {
	arches := rhel10FamilyDistros[0].distro.ListArches()
	assert.Equal(t, []string{"aarch64", "ppc64le", "s390x", "x86_64"}, arches)
}

func TestRH10Rhel10_GetArch(t *testing.T) {
	arches := []struct {
		name                  string
		errorExpected         bool
		errorExpectedInCentos bool
	}{
		{
			name: "x86_64",
		},
		{
			name: "aarch64",
		},
		{
			name: "ppc64le",
		},
		{
			name: "s390x",
		},
		{
			name:          "foo-arch",
			errorExpected: true,
		},
	}

	for _, dist := range rhel10FamilyDistros {
		t.Run(dist.name, func(t *testing.T) {
			for _, a := range arches {
				actualArch, err := dist.distro.GetArch(a.name)
				if a.errorExpected || (a.errorExpectedInCentos && dist.name == "centos") {
					assert.Nil(t, actualArch)
					assert.Error(t, err)
				} else {
					assert.Equal(t, a.name, actualArch.Name())
					assert.NoError(t, err)
				}
			}
		})
	}
}

func TestRH10Rhel10_Name(t *testing.T) {
	distro := rhel10FamilyDistros[0].distro
	assert.Equal(t, "rhel-10.0", distro.Name())
}

func TestRH10Rhel10_ModulePlatformID(t *testing.T) {
	distro := rhel10FamilyDistros[0].distro
	assert.Equal(t, "platform:el10", distro.ModulePlatformID())
}

func TestRH10Rhel10_KernelOption(t *testing.T) {
	distro_test_common.TestDistro_KernelOption(t, rhel10FamilyDistros[0].distro)
}

func TestRH10Rhel10_KernelOption_NoIfnames(t *testing.T) {
	for _, distroName := range []string{"rhel-10.0", "centos-10"} {
		distro := generic.DistroFactory(distroName)
		for _, archName := range distro.ListArches() {
			arch, err := distro.GetArch(archName)
			assert.NoError(t, err)
			for _, imgTypeName := range arch.ListImageTypes() {
				imgType, err := arch.GetImageType(imgTypeName)
				assert.NoError(t, err)
				imgCfg := imgType.(*generic.ImageType).GetDefaultImageConfig()
				if imgCfg != nil {
					assert.NotContains(t, imgCfg.KernelOptions, "net.ifnames=0", "type %s contains unwanted net.ifnames=0", imgType.Name())
				}
			}
		}
	}
}

func TestRH10Distro_CustomFileSystemManifestError(t *testing.T) {
	r10distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/etc",
				},
			},
		},
	}
	for _, archName := range r10distro.ListArches() {
		arch, _ := r10distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, _, err := imgType.Manifest(&bp, distro.ImageOptions{}, nil, nil)
			assert.EqualError(t, err, "The following errors occurred while setting up custom mountpoints:\npath \"/etc\" is not allowed")
		}
	}
}

func TestRH10Distro_TestRootMountPoint(t *testing.T) {
	r10distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/",
				},
			},
		},
	}
	for _, archName := range r10distro.ListArches() {
		arch, _ := r10distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, _, err := imgType.Manifest(&bp, distro.ImageOptions{}, nil, nil)
			assert.NoError(t, err)
		}
	}
}

func TestRH10Distro_CustomFileSystemSubDirectories(t *testing.T) {
	r10distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/var/log",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var/log/audit",
				},
			},
		},
	}
	for _, archName := range r10distro.ListArches() {
		arch, _ := r10distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, _, err := imgType.Manifest(&bp, distro.ImageOptions{}, nil, nil)
			if strings.HasPrefix(imgTypeName, "edge-") {
				continue
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestRH10Distro_MountpointsWithArbitraryDepthAllowed(t *testing.T) {
	r10distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/var/a",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var/a/b",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var/a/b/c",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var/a/b/c/d",
				},
			},
		},
	}
	for _, archName := range r10distro.ListArches() {
		arch, _ := r10distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, _, err := imgType.Manifest(&bp, distro.ImageOptions{}, nil, nil)
			if strings.HasPrefix(imgTypeName, "edge-") {
				continue
			} else {
				assert.NoError(t, err)
			}
		}
	}
}

func TestRH10Distro_DirtyMountpointsNotAllowed(t *testing.T) {
	r10distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "//",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var//",
				},
				{
					MinSize:    1024,
					Mountpoint: "/var//log/audit/",
				},
			},
		},
	}
	for _, archName := range r10distro.ListArches() {
		arch, _ := r10distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, _, err := imgType.Manifest(&bp, distro.ImageOptions{}, nil, nil)
			assert.EqualError(t, err, "The following errors occurred while setting up custom mountpoints:\npath \"//\" must be canonical\npath \"/var//\" must be canonical\npath \"/var//log/audit/\" must be canonical")
		}
	}
}

func TestRH10Distro_CustomUsrPartitionNotLargeEnough(t *testing.T) {
	r10distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/usr",
				},
			},
		},
	}
	for _, archName := range r10distro.ListArches() {
		arch, _ := r10distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			_, _, err := imgType.Manifest(&bp, distro.ImageOptions{}, nil, nil)
			assert.NoError(t, err)
		}
	}
}

func TestRH10DiskAndFilesystemCustomizationsError(t *testing.T) {
	// simple test that checks that disk customizations are allowed
	r8distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Filesystem: []blueprint.FilesystemCustomization{
				{
					MinSize:    1024,
					Mountpoint: "/home",
				},
			},
			Disk: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type: "plain",
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
							Label:      "root",
							FSType:     "ext4",
						},
					},
				},
			},
		},
	}

	// these produce error message and are tested elsewhere
	skipTest := map[string]bool{
		"edge-commit":               true,
		"edge-container":            true,
		"edge-installer":            true,
		"edge-simplified-installer": true,
		"azure-eap7-rhui":           true,
		"edge-vsphere":              true,
		"edge-raw-image":            true,
		"edge-ami":                  true,
	}

	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			options := distro.ImageOptions{}
			_, _, err := imgType.Manifest(&bp, options, nil, nil)
			if skipTest[imgTypeName] {
				continue
			}
			assert.EqualError(t, err, "partitioning customizations cannot be used with custom filesystems (mountpoints)")
		}
	}
}

func TestRH10NoDiskCustomizationsNoError(t *testing.T) {
	// simple test that checks that disk customizations are allowed
	r8distro := rhel10FamilyDistros[0].distro
	bp := blueprint.Blueprint{
		Customizations: &blueprint.Customizations{
			Disk: &blueprint.DiskCustomization{
				Partitions: []blueprint.PartitionCustomization{
					{
						Type: "plain",
						FilesystemTypedCustomization: blueprint.FilesystemTypedCustomization{
							Mountpoint: "/",
							Label:      "root",
							FSType:     "ext4",
						},
					},
				},
			},
		},
	}

	// these produce error message and are tested elsewhere
	skipTest := map[string]bool{
		"edge-commit":               true,
		"edge-container":            true,
		"edge-installer":            true,
		"edge-simplified-installer": true,
		"azure-eap7-rhui":           true,
		"edge-vsphere":              true,
		"edge-raw-image":            true,
		"edge-ami":                  true,
	}

	for _, archName := range r8distro.ListArches() {
		arch, _ := r8distro.GetArch(archName)
		for _, imgTypeName := range arch.ListImageTypes() {
			imgType, _ := arch.GetImageType(imgTypeName)
			options := distro.ImageOptions{}
			_, _, err := imgType.Manifest(&bp, options, nil, nil)
			if skipTest[imgTypeName] {
				continue
			}
			assert.NoError(t, err)
		}
	}
}
