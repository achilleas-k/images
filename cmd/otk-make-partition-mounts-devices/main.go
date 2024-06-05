package main

import (
	"fmt"
	"io"
	"os"

	"github.com/osbuild/images/internal/otk"
	"github.com/osbuild/images/pkg/osbuild"
)

type Input struct {
	Internal otk.PartitionInternal `json:"internal"`
}

type Output struct {
	Mounts  []osbuild.Mount           `json:"mounts"`
	Devices map[string]osbuild.Device `json:"devices"`
}

func run(r io.Reader, w io.Writer) error {
	return nil
}

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "error: %v", err.Error())
		os.Exit(1)
	}
}
