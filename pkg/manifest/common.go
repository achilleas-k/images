package manifest

import (
	"strings"

	"github.com/osbuild/images/pkg/disk"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
)

// filesystemConfigStages generates either an org.osbuild.fstab stage or a
// collection of org.osbuild.systemd.unit.create stages for .mount and .swap
// units (and an org.osbuild.systemd stage to enable them) depending on the
// pipeline configuration.
func filesystemConfigStages(pt *disk.PartitionTable, mountUnits bool) ([]*osbuild.Stage, error) {
	if mountUnits {
		return osbuild.GenSystemdMountStages(pt)
	} else {
		opts, err := osbuild.NewFSTabStageOptions(pt)
		if err != nil {
			return nil, err
		}
		return []*osbuild.Stage{osbuild.NewFSTabStage(opts)}, nil
	}
}

func grubStage(p *OS, pt *disk.PartitionTable, kernelOptions []string) *osbuild.Stage {
	if p.NoBLS {
		// BLS entries not supported: use grub2.legacy
		id := "76a22bf4-f153-4541-b6c7-0332c0dfaeac"
		product := osbuild.GRUB2Product{
			Name:    p.OSProduct,
			Version: p.OSVersion,
			Nick:    p.OSNick,
		}

		_, err := rpmmd.GetVerStrFromPackageSpecList(p.packageSpecs, "dracut-config-rescue")
		hasRescue := err == nil
		return osbuild.NewGrub2LegacyStage(
			osbuild.NewGrub2LegacyStageOptions(
				p.Grub2Config,
				p.PartitionTable,
				kernelOptions,
				p.platform.GetBIOSPlatform(),
				p.platform.GetUEFIVendor(),
				osbuild.MakeGrub2MenuEntries(id, p.kernelVer, product, hasRescue),
			),
		)
	} else {
		options := osbuild.NewGrub2StageOptions(pt,
			strings.Join(kernelOptions, " "),
			p.kernelVer,
			p.platform.GetUEFIVendor() != "",
			p.platform.GetBIOSPlatform(),
			p.platform.GetUEFIVendor(), false)
		if cfg := p.Grub2Config; cfg != nil {
			// TODO: don't store Grub2Config in OSPipeline, making the overrides unnecessary
			// grub2.Config.Default is owned and set by `NewGrub2StageOptionsUnified`
			// and thus we need to preserve it
			if options.Config != nil {
				cfg.Default = options.Config.Default
			}

			options.Config = cfg
		}
		if p.KernelOptionsBootloader {
			options.WriteCmdLine = nil
			if options.UEFI != nil {
				options.UEFI.Unified = false
			}
		}
		return osbuild.NewGRUB2Stage(options)
	}
}
