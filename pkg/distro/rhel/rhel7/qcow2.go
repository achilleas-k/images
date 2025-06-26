package rhel7

import (
	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/subscription"
	"github.com/osbuild/images/pkg/datasizes"
	"github.com/osbuild/images/pkg/distro/rhel"
	"github.com/osbuild/images/pkg/imageconfig"
	"github.com/osbuild/images/pkg/osbuild"
)

func mkQcow2ImgType() *rhel.ImageType {
	it := rhel.NewImageType(
		"qcow2",
		"disk.qcow2",
		"application/x-qemu-disk",
		packageSetLoader,
		rhel.DiskImage,
		[]string{"build"},
		[]string{"os", "image", "qcow2"},
		[]string{"qcow2"},
	)

	// all RHEL 7 images should use sgdisk
	it.DiskImagePartTool = common.ToPtr(osbuild.PTSgdisk)

	it.Bootable = true
	it.DefaultSize = 10 * datasizes.GibiByte
	it.DefaultImageConfig = qcow2DefaultImgConfig
	it.BasePartitionTables = defaultBasePartitionTables

	return it
}

var qcow2DefaultImgConfig = &imageconfig.ImageConfig{
	DefaultTarget:       common.ToPtr("multi-user.target"),
	SELinuxForceRelabel: common.ToPtr(true),
	UpdateDefaultKernel: common.ToPtr(true),
	DefaultKernel:       common.ToPtr("kernel"),
	Sysconfig: &imageconfig.Sysconfig{
		Networking:                  true,
		NoZeroConf:                  true,
		CreateDefaultNetworkScripts: true,
	},
	RHSMConfig: map[subscription.RHSMStatus]*subscription.RHSMConfig{
		subscription.RHSMConfigNoSubscription: {
			YumPlugins: subscription.SubManDNFPluginsConfig{
				ProductID: subscription.DNFPluginConfig{
					Enabled: common.ToPtr(false),
				},
				SubscriptionManager: subscription.DNFPluginConfig{
					Enabled: common.ToPtr(false),
				},
			},
		},
	},
	KernelOptions: []string{"console=tty0", "console=ttyS0,115200n8", "no_timer_check", "net.ifnames=0", "crashkernel=auto"},
}
