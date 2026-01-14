package testcontainers

import (
	"embed"
	"io/fs"

	"github.com/osbuild/images/pkg/containerregistry"
)

//go:embed *.json
var FS embed.FS

func New() (*containerregistry.ContainerRegistry, error) {
	return containerregistry.New(nil, []fs.FS{FS})
}
