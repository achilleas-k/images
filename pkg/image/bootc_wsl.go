package image

import (
	"math/rand"

	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/customizations/fsnode"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/runner"
)

type BootcWSL struct {
	Base

	ContainerSource      *container.SourceSpec
	BuildContainerSource *container.SourceSpec

	// Customizations
	OSCustomizations manifest.OSCustomizations
}

func NewBootcWSL(platform platform.Platform, filename string, container container.SourceSpec, buildContainer container.SourceSpec) *BootcWSL {
	return &BootcWSL{
		Base:                 NewBase("bootc-wsl", platform, filename),
		ContainerSource:      &container,
		BuildContainerSource: &buildContainer,
	}
}

func (img *BootcWSL) InstantiateManifestFromContainers(m *manifest.Manifest,
	containers []container.SourceSpec,
	runner runner.Runner,
	rng *rand.Rand) error {

	policy := img.OSCustomizations.SELinux
	if img.OSCustomizations.BuildSELinux != "" {
		policy = img.OSCustomizations.BuildSELinux
	}

	var copyFilesFrom map[string][]string
	var ensureDirs []*fsnode.Directory

	var customSourcePipeline = ""
	if *img.ContainerSource != *img.BuildContainerSource {
		// If we're using a different build container from the target container then we copy
		// the bootc customization file directories from the target container. This includes the
		// bootc install customization, and /usr/lib/ostree/prepare-root.conf which configures
		// e.g. composefs and fs-verity setup.
		//
		// To ensure that these copies never fail we also create the source and target
		// directories as needed.

		pipelineName := "target"
		// files to copy have slash at end to copy directory contents, not directory itself
		copyFiles := []string{"/usr/lib/bootc/install/", "/usr/lib/ostree/"}
		ensureDirPaths := []string{"/usr/lib/bootc/install", "/usr/lib/ostree"}

		copyFilesFrom = map[string][]string{pipelineName: copyFiles}
		for _, path := range ensureDirPaths {
			// Note: Mode/User/Group must be nil here to make  GenDirectoryNodesStages use dirExistOk
			dir, err := fsnode.NewDirectory(path, nil, nil, nil, true)
			if err != nil {
				return err
			}
			ensureDirs = append(ensureDirs, dir)
		}

		targetContainers := []container.SourceSpec{*img.ContainerSource}
		targetBuildPipeline := manifest.NewBuildFromContainer(m, runner, targetContainers,
			&manifest.BuildOptions{
				PipelineName:       pipelineName,
				ContainerBuildable: true,
				SELinuxPolicy:      policy,
				EnsureDirs:         ensureDirs,
			})
		targetBuildPipeline.Checkpoint()

		customSourcePipeline = targetBuildPipeline.Name()
	}

	buildContainers := []container.SourceSpec{*img.BuildContainerSource}
	buildPipeline := manifest.NewBuildFromContainer(m, runner, buildContainers,
		&manifest.BuildOptions{
			ContainerBuildable: true,
			SELinuxPolicy:      policy,
			CopyFilesFrom:      copyFilesFrom,
			EnsureDirs:         ensureDirs,
		})

	buildPipeline.Checkpoint()

	println(customSourcePipeline)

	return nil
}
