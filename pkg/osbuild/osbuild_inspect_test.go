package osbuild_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"testing"

	"github.com/osbuild/images/pkg/osbuild"
	"github.com/osbuild/images/pkg/rpmmd"
	"github.com/stretchr/testify/assert"
)

// create a simple manifest with one or more stages
func makeSimpleManifest(stages ...*osbuild.Stage) *osbuild.Manifest {
	sources, err := osbuild.GenSources(
		[]rpmmd.PackageSpec{
			{
				Name:     "pkg",
				Checksum: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
			},
		},
		nil, nil, nil)
	if err != nil {
		panic(err)
	}

	pl := osbuild.Pipeline{
		Name: "test",
	}
	pl.AddStages(stages...)
	m := &osbuild.Manifest{
		Version:   "2",
		Pipelines: []osbuild.Pipeline{pl},
		Sources:   sources,
	}
	return m
}

func inspectManifest(m *osbuild.Manifest) error {
	ms, err := json.Marshal(m)
	if err != nil {
		return err
	}
	cmd := exec.Command(
		"osbuild",
		"--inspect",
		"-",
	)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return err
	}

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Start(); err != nil {
		return err
	}

	if _, err := stdin.Write(ms); err != nil {
		return err
	}

	if err := stdin.Close(); err != nil {
		return err
	}

	return cmd.Wait()
}

func TestRPMStageInspect(t *testing.T) {
	options := &osbuild.RPMStageOptions{}
	specs := []rpmmd.PackageSpec{
		{
			Name:     "pkg",
			Checksum: "sha256:ffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffffff",
		},
	}
	inputs := osbuild.NewRpmStageSourceFilesInputs(specs)
	rpmstage := osbuild.NewRPMStage(options, inputs)
	m := makeSimpleManifest(rpmstage)
	assert.NoError(t, inspectManifest(m))
}

func TestOVFStage(t *testing.T) {
	stage := osbuild.NewOVFStage(&osbuild.OVFStageOptions{Vmdk: "file.vmdk"})
	m := makeSimpleManifest(stage)
	assert.NoError(t, inspectManifest(m))
}

func _TestBootupdGenInspect(t *testing.T) {
	stage := osbuild.NewBootupdGenMetadataStage()
	m := makeSimpleManifest(stage)
	assert.NoError(t, inspectManifest(m))
}
