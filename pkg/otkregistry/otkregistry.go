// otkregistry is a distro registry composed of distributions defined using otk yaml files.
package otkregistry

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/otkdistro"
)

type Registry struct {
	root          string
	Distributions []distro.Distro
}

func New(root string) (*Registry, error) {
	r := &Registry{
		root: root,
	}

	entries, err := os.ReadDir(root)
	if err != nil {
		return nil, fmt.Errorf("error reading otk root directory %q: %w", root, err)
	}

	for _, entry := range entries {
		path := filepath.Join(root, entry.Name())
		d, err := otkdistro.New(path)
		if err != nil {
			return nil, fmt.Errorf("error reading otk distro root directory %q: %w", path, err)
		}
		r.Distributions = append(r.Distributions, d)
	}

	return r, nil
}
