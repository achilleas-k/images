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
	Depsolve          distro.DepsolveFunc
	ResolveContainers distro.ContainerResolveFunc
	ResolveCommits    distro.CommitResolveFunc
}

// Generator can generate an osbuild manifest from a given repository
// and options.
type Generator struct {
	cacheDir string
	out      io.Writer

	depsolve          distro.DepsolveFunc
	resolveContainers distro.ContainerResolveFunc
	resolveCommits    distro.CommitResolveFunc
	sbomWriter        SBOMWriterFunc
	warningsOutput    io.Writer

	reporegistry *reporegistry.RepoRegistry

	rpmDownloader osbuild.RpmDownloader

	customSeed    *int64
	overrideRepos []rpmmd.RepoConfig
}

// New will create a new manifest generator
func New(reporegistry *reporegistry.RepoRegistry, opts *Options) (*Generator, error) {
	if opts == nil {
		opts = &Options{}
	}
	mg := &Generator{
		reporegistry: reporegistry,

		cacheDir:          opts.Cachedir,
		out:               opts.Output,
		depsolve:          opts.Depsolve,
		resolveContainers: opts.ResolveContainers,
		resolveCommits:    opts.ResolveCommits,
		rpmDownloader:     opts.RpmDownloader,
		sbomWriter:        opts.SBOMWriter,
		warningsOutput:    opts.WarningsOutput,
		customSeed:        opts.CustomSeed,
		overrideRepos:     opts.OverrideRepos,
	}

	return mg, nil
}

// Generate will generate a new manifest for the given distro/imageType/arch
// combination.
func (mg *Generator) Generate(bp *blueprint.Blueprint, dist distro.Distro, imgType distro.ImageType, a distro.Arch, imgOpts *distro.ImageOptions) (err error) {
	if imgOpts == nil {
		imgOpts = &distro.ImageOptions{}
	}

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
		Depsolve:          mg.depsolve,
		ResolveContainers: mg.resolveContainers,
		ResolveCommits:    mg.resolveCommits,
	}

	// set fallbacks for any unset resolvers
	if resolvers.Depsolve == nil {
		defaultDepsolver, err := DefaultDepsolver(mg.cacheDir, dist, a.Name())
		if err != nil {
			return err
		}

		// Always generate Spdx SBOMs for now, this makes the default depsolve
		// slightly slower but it means we need no extra argument here to
		// select the SBOM type. Once we have more types than Spdx of course we
		// need to add a option to select the type.
		defaultDepsolver.SetSBOMType(sbom.StandardTypeSpdx)
		resolvers.Depsolve = defaultDepsolver.DepsolveAll
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
	fmt.Fprintf(mg.out, "%s\n", mf)

	if mg.sbomWriter != nil {
		// XXX: this is very similar to
		// osbuild-composer:jobimpl-osbuild.go, see if code
		// can be shared

		xxx
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

// DefaultDepsolver provides a default initialisation of [dnfjson.Solver].
// It should rarely be necessary to use it directly and will be used by default
// by manifestgen (unless overriden)
func DefaultDepsolver(cacheDir string, d distro.Distro, arch string) (*dnfjson.Solver, error) {
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
	DepsolveFunc func(cacheDir string, packageSets map[string][]rpmmd.PackageSet, d distro.Distro, arch string) (map[string]dnfjson.DepsolveResult, error)

	ContainerResolverFunc func(containerSources map[string][]container.SourceSpec, archName string) (map[string][]container.Spec, error)

	CommitResolverFunc func(commitSources map[string][]ostree.SourceSpec) (map[string][]ostree.CommitSpec, error)

	SBOMWriterFunc func(filename string, content io.Reader, docType sbom.StandardType) error
)
