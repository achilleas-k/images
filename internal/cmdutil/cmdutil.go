// cmdutil contains common utility functions for the command line tools defined in cmd/.
package cmdutil

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/osbuild/images/pkg/blueprint"
	"github.com/osbuild/images/pkg/ostree"
	"github.com/osbuild/images/pkg/rpmmd"
)

// FilterRepos returns the subset of the provided repositories that apply to a
// given image type. This is generally when repositories are defined for
// testing some image types and we don't want them being used for all builds.
func FilterRepos(repos []rpmmd.RepoConfig, typeName string) []rpmmd.RepoConfig {
	filtered := make([]rpmmd.RepoConfig, 0)
	for _, repo := range repos {
		if len(repo.ImageTypeTags) == 0 {
			filtered = append(filtered, repo)
		} else {
			for _, tt := range repo.ImageTypeTags {
				if tt == typeName {
					filtered = append(filtered, repo)
					break
				}
			}
		}
	}
	return filtered
}

type DistroArchRepoMap map[string]map[string][]rpmmd.RepoConfig

func LoadRepoMap() DistroArchRepoMap {
	reposDir := "./test/data/repositories/"
	darm := make(DistroArchRepoMap)
	filelist, err := os.ReadDir(reposDir)
	if err != nil {
		panic(err)
	}
	for _, file := range filelist {
		filename := file.Name()
		if !strings.HasSuffix(filename, ".json") {
			continue
		}
		reposFilepath := filepath.Join(reposDir, filename)
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
		distro := strings.TrimSuffix(filename, filepath.Ext(filename))
		darm[distro] = repos
	}
	return darm
}

type BuildConfig struct {
	Name      string               `json:"name"`
	OSTree    *ostree.ImageOptions `json:"ostree,omitempty"`
	Blueprint *blueprint.Blueprint `json:"blueprint,omitempty"`
	Depends   BuildDependency      `json:"depends,omitempty"`
}

type BuildDependency struct {
	Config    string `json:"config"`
	ImageType string `json:"image-type"`
}

func LoadConfig(path string) BuildConfig {
	fp, err := os.Open(path)
	if err != nil {
		panic(fmt.Sprintf("failed to open config %q: %s", path, err.Error()))
	}
	defer fp.Close()

	dec := json.NewDecoder(fp)
	dec.DisallowUnknownFields()
	var conf BuildConfig

	if err := dec.Decode(&conf); err != nil {
		panic(fmt.Sprintf("failed to unmarshal config %q: %s", path, err.Error()))
	}
	if dec.More() {
		panic(fmt.Sprintf("multiple configuration objects or extra data found in %q", path))
	}
	return conf
}
