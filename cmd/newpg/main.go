package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/osbuild/images/internal/dnfjson"
	"github.com/osbuild/images/internal/environment"
	"github.com/osbuild/images/internal/workload"
	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distro/fedora"
	"github.com/osbuild/images/pkg/new/components"
	"github.com/osbuild/images/pkg/platform"
	"github.com/osbuild/images/pkg/rpmmd"
)

func main() {
	ic := components.NewConfig()

	plat := &platform.X86{
		BIOS:       true,
		UEFIVendor: "fedora",
		BasePlatform: platform.BasePlatform{
			ImageFormat: platform.FORMAT_RAW,
		},
	}
	ic.SetPlatform(plat)

	env := &environment.EC2{}
	ic.SetEnvironment(env)

	d := fedora.NewF38()
	ic.SetDistro(d)

	wl := &workload.Custom{}
	ic.SetWorkload(wl)

	it := ic.ImageType()
	options := distro.ImageOptions{Size: 0}

	bp := &blueprint.Blueprint{
		Packages: []blueprint.Package{
			{Name: "kernel"},
		},
	}
	manifest, _, err := it.Manifest(bp, options, readRepos(d.Name(), plat.GetArch().String()), 0)
	if err != nil {
		panic(err)
	}

	solver := dnfjson.NewSolver(d.ModulePlatformID(), d.Releasever(), plat.GetArch().String(), d.Name(), "/tmp/rpmmd")
	depsolvedSets := make(map[string][]rpmmd.PackageSpec)
	for name, pkgSet := range manifest.GetPackageSetChains() {
		res, err := solver.Depsolve(pkgSet)
		if err != nil {
			panic(err)
		}
		depsolvedSets[name] = res
	}

	mf, err := manifest.Serialize(depsolvedSets, nil, nil)
	if err != nil {
		panic(err)
	}

	mfJson, err := json.MarshalIndent(mf, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(mfJson))
}

func readRepos(distro, arch string) []rpmmd.RepoConfig {
	reposDir := "./test/data/repositories/"
	reposFilepath := filepath.Join(reposDir, distro+".json")
	fp, err := os.Open(reposFilepath)
	if err != nil {
		panic(err)
	}
	defer fp.Close()
	data, err := io.ReadAll(fp)
	if err != nil {
		panic(err)
	}
	repos := make(map[string][]rpmmd.RepoConfig)
	if err := json.Unmarshal(data, &repos); err != nil {
		panic(err)
	}
	return repos[arch]
}
