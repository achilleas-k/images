package manifestgen

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/container"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/dnfjson"
	"github.com/osbuild/images/pkg/manifest"
	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/reporegistry"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/osbuild/images/pkg/sbom"
)

const (
	defaultDepsolverSBOMType = sbom.StandardTypeSpdx
	defaultSBOMExt           = "spdx.json"

	defaultDepsolveCacheDir = "osbuild-depsolve-dnf"
)

// Options contains the optional settings for the manifest generation.
// For unset values defaults will be used.
type Options struct {
	Cachedir string
	// Output is the writer that the generated osbuild manifest will
	// written to.
	Output io.Writer

	RpmDownloader osbuild.RpmDownloader

	// SBOMWriter will be called for each generated SBOM the
	// filename contains the suggest filename string and the
	// content can be read
	SBOMWriter SBOMWriterFunc

	// WarningsOutput will receive any warnings that are part of
	// the manifest generation. If it is unset any warnings will
	// generate an error.
	WarningsOutput io.Writer

	// DepsolveWarningsOutput will receive any warnings that are
	// part of the depsolving step. If it is unset output ends up
	// on the default stdout/stderr.
	DepsolveWarningsOutput io.Writer

	// CustomSeed overrides the default rng seed, this is mostly
	// useful for testing
	CustomSeed *int64

	// OverrideRepos overrides the default repository selection.
	// This is mostly useful for testing
	OverrideRepos []rpmmd.RepoConfig

	// ArchName is the architecture name for the target image.
	ArchName string

	// Custom "solver" functions, if unset the defaults will be
	// used. Only needed for specialized use-cases.
	Depsolver         distro.DepsolveFunc
	ContainerResolver distro.ContainerResolveFunc
	CommitResolver    distro.CommitResolveFunc

	// Use the a bootstrap container to buildroot (useful for e.g.
	// cross-arch or cross-distro builds)
	UseBootstrapContainer bool
}

// Generator can generate an osbuild manifest from a given repository
// and options.
type Generator struct {
	cacheDir string
	out      io.Writer

	depsolver              distro.DepsolveFunc
	containerResolver      distro.ContainerResolveFunc
	commitResolver         distro.CommitResolveFunc
	sbomWriter             SBOMWriterFunc
	warningsOutput         io.Writer
	depsolveWarningsOutput io.Writer

	reporegistry *reporegistry.RepoRegistry

	rpmDownloader osbuild.RpmDownloader

	customSeed    *int64
	overrideRepos []rpmmd.RepoConfig

	useBootstrapContainer bool
}

// New will create a new manifest generator
func New(reporegistry *reporegistry.RepoRegistry, opts *Options) (*Generator, error) {
	if opts == nil {
		opts = &Options{}
	}
	mg := &Generator{
		reporegistry: reporegistry,

		cacheDir:               opts.Cachedir,
		out:                    opts.Output,
		depsolver:              opts.Depsolver,
		containerResolver:      opts.ContainerResolver,
		commitResolver:         opts.CommitResolver,
		rpmDownloader:          opts.RpmDownloader,
		sbomWriter:             opts.SBOMWriter,
		warningsOutput:         opts.WarningsOutput,
		depsolveWarningsOutput: opts.DepsolveWarningsOutput,
		customSeed:             opts.CustomSeed,
		overrideRepos:          opts.OverrideRepos,
		useBootstrapContainer:  opts.UseBootstrapContainer,
	}

	return mg, nil
}

// Generate will generate a new manifest for the given distro/imageType/arch
// combination.
func (mg *Generator) Generate(bp *blueprint.Blueprint, dist distro.Distro, imgType distro.ImageType, a distro.Arch, imgOpts *distro.ImageOptions) (err error) {
	if imgOpts == nil {
		imgOpts = &distro.ImageOptions{}
	}
	imgOpts.UseBootstrapContainer = mg.useBootstrapContainer

	var repos []rpmmd.RepoConfig
	if mg.overrideRepos != nil {
		repos = mg.overrideRepos
	} else {
		repos, err = mg.reporegistry.ReposByImageTypeName(dist.Name(), a.Name(), imgType.Name())
		if err != nil {
			return err
		}
	}

	resolvers := distro.Resolvers{
		Depsolve:          mg.depsolver,
		ResolveContainers: mg.containerResolver,
		ResolveCommits:    mg.commitResolver,
	}

	// set fallbacks for any unset resolvers

	// keep the depsolve results so we can get the sbom
	var depsolved map[string]dnfjson.DepsolveResult
	if resolvers.Depsolve == nil {
		defaultDepsolver, err := DefaultDepsolver(mg.cacheDir, mg.depsolveWarningsOutput, dist, a.Name())
		if err != nil {
			return err
		}

		// Always generate Spdx SBOMs for now, this makes the default depsolve
		// slightly slower but it means we need no extra argument here to
		// select the SBOM type. Once we have more types than Spdx of course we
		// need to add a option to select the type.
		defaultDepsolver.SetSBOMType(sbom.StandardTypeSpdx)
		resolvers.Depsolve = defaultDepsolver.DepsolveAll
		resolvers.Depsolve = func(pkgSetsMap map[string][]rpmmd.PackageSet) (map[string]dnfjson.DepsolveResult, error) {
			res, err := defaultDepsolver.DepsolveAll(pkgSetsMap)
			if err != nil {
				return nil, err
			}
			depsolved = res
			return res, nil
		}
	}

	if resolvers.ResolveContainers == nil {
		resolvers.ResolveContainers = container.NewResolver(a.Name()).ResolveAll
	}

	if resolvers.ResolveCommits == nil {
		resolvers.ResolveCommits = ostree.ResolveAll
	}

	opts := &manifest.SerializeOptions{
		RpmDownloader: mg.rpmDownloader,
	}

	// To support "user" a.k.a. "3rd party" repositories, these will have to be
	// added to the repos with <repo_item>.PackageSets set to the "payload"
	// pipeline names for the given image type, see e.g.
	// distro/rhel/imagetype.go:Manifest()
	mf, warnings, err := imgType.SerializedManifest(bp, *imgOpts, repos, resolvers, opts, mg.customSeed)
	if err != nil {
		return err
	}
	if len(warnings) > 0 {
		warn := strings.Join(warnings, "\n")
		if mg.warningsOutput != nil {
			fmt.Fprint(mg.warningsOutput, warn)
		} else {
			return fmt.Errorf("Warnings during manifest creation:\n%v", warn)
		}
	}
	containerSpecs, err := mg.containerResolver(preManifest.GetContainerSourceSpecs(), a.Name())
	if err != nil {
		return err
	}
	commitSpecs, err := mg.commitResolver(preManifest.GetOSTreeSourceSpecs())
	if err != nil {
		return err
	}
	opts := &manifest.SerializeOptions{
		RpmDownloader: mg.rpmDownloader,
	}
	mf, err := preManifest.Serialize(depsolved, containerSpecs, commitSpecs, opts)
	if err != nil {
		return err
	}
	fmt.Fprintf(mg.out, "%s\n", mf)

	if mg.sbomWriter != nil {
		// XXX: this is very similar to
		// osbuild-composer:jobimpl-osbuild.go, see if code
		// can be shared

		// NOTE: With new solver way, we can't get the sbom ...
		for plName, depsolvedPipeline := range depsolved {
			pipelinePurpose := "unknown"
			switch {
			case slices.Contains(imgType.PayloadPipelines(), plName):
				pipelinePurpose = "image"
			case slices.Contains(imgType.BuildPipelines(), plName):
				pipelinePurpose = "buildroot"
			}
			// XXX: sync with image-builder-cli:build.go name generation - can we have a shared helper?
			imageName := fmt.Sprintf("%s-%s-%s", dist.Name(), imgType.Name(), a.Name())
			sbomDocOutputFilename := fmt.Sprintf("%s.%s-%s.%s", imageName, pipelinePurpose, plName, defaultSBOMExt)

			var buf bytes.Buffer
			enc := json.NewEncoder(&buf)
			if err := enc.Encode(depsolvedPipeline.SBOM.Document); err != nil {
				return err
			}
			if err := mg.sbomWriter(sbomDocOutputFilename, &buf, depsolvedPipeline.SBOM.DocType); err != nil {
				return err
			}
		}
	}

	return nil
}

func xdgCacheHome() (string, error) {
	xdgCacheHome := os.Getenv("XDG_CACHE_HOME")
	if xdgCacheHome != "" {
		return xdgCacheHome, nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".cache"), nil
}

// DefaultDepsolver provides a default implementation for depsolving.
// It should rarely be necessary to use it directly and will be used
// by default by manifestgen (unless overriden)
func DefaultDepsolver(cacheDir string, depsolveWarningsOutput io.Writer, d distro.Distro, arch string) (*dnfjson.Solver, error) {
	if cacheDir == "" {
		xdgCacheHomeDir, err := xdgCacheHome()
		if err != nil {
			return nil, err
		}
		cacheDir = filepath.Join(xdgCacheHomeDir, defaultDepsolveCacheDir)
	}

	return dnfjson.NewSolver(d.ModulePlatformID(), d.Releasever(), arch, d.Name(), cacheDir), nil
}

type (
	DepsolveFunc func(cacheDir string, depsolveWarningsOutput io.Writer, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]dnfjson.DepsolveResult, error)

	ContainerResolverFunc func(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error)

	CommitResolverFunc func(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error)

	SBOMWriterFunc func(filename string, content io.Reader, docType sbom.StandardType) error
)
