package containerregistry

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"strings"
)

// ContainerRegistry represents a collection of architecture-specific container
// references.
type ContainerRegistry struct {
	refs map[string]map[string]string
}

// New returns a new ContainerRegistry instance with the data loaded from the
// given configPaths and configFS instance. The order is important here, first
// the paths are tried, then the FSes.
//
// Note that the confPaths must point directly to the directory with the json
// files.
func New(configPaths []string, configFS []fs.FS) (*ContainerRegistry, error) {
	refs, err := LoadAllRefs(configPaths, configFS)
	if err != nil {
		return nil, err
	}

	return &ContainerRegistry{refs}, nil
}

// LoadAllRefs loads all container references from the given list of paths.
func LoadAllRefs(confPaths []string, confFSes []fs.FS) (map[string]map[string]string, error) {
	var mergedFSes []fs.FS

	for _, path := range confPaths {
		mergedFSes = append(mergedFSes, os.DirFS(path))
	}
	mergedFSes = append(mergedFSes, confFSes...)

	allRefs := make(map[string]map[string]string)
	for _, fsys := range mergedFSes {
		fileEntries, err := fs.ReadDir(fsys, ".")
		if os.IsNotExist(err) {
			continue
		} else if err != nil {
			return nil, err
		}

		for _, fileEntry := range fileEntries {
			// Skip all directories
			if fileEntry.IsDir() {
				continue
			}

			// use the name of each file as the short name for this collection
			// of container refs
			if strings.HasSuffix(fileEntry.Name(), ".json") {
				// The name of each entry is the base filename with bootc:
				// prepended to it. This is needed to identify
				// container/bootc-based variants of the virtual bootc distro.
				shortName := strings.TrimSuffix(fileEntry.Name(), ".json")
				shortName = fmt.Sprintf("bootc:%s", shortName)
				fp, err := fsys.Open(fileEntry.Name())
				if err != nil {
					return nil, fmt.Errorf("failed to load container refs from %q: %w", fileEntry.Name(), err)
				}

				decoder := json.NewDecoder(fp)
				decoder.DisallowUnknownFields()

				archRefs := make(map[string]string)
				decoder.Decode(&archRefs)
				allRefs[shortName] = archRefs
			}
		}
	}
	return allRefs, nil
}

// List returns a list of all names which have a container ref defined in the
// registry.
func (r *ContainerRegistry) List() []string {
	names := make([]string, 0, len(r.refs))
	for name := range r.refs {
		names = append(names, name)
	}
	return names
}
