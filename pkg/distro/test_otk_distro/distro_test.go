package test_otk_distro_test

import (
	"fmt"
	"testing"

	"github.com/osbuild/images/pkg/distro/test_otk_distro"
	"github.com/stretchr/testify/assert"
)

func TestDistroLoad(t *testing.T) {
	assert := assert.New(t)

	distro := test_otk_distro.New("../../../test/data/otk/fakedistro")
	assert.Equal("fakedistro", distro.Name())

	archImageTypes := make([]string, 0)
	for _, archName := range distro.ListArches() {
		arch, err := distro.GetArch(archName)
		assert.NoError(err)

		for _, imageTypeName := range arch.ListImageTypes() {
			archImageTypes = append(archImageTypes, fmt.Sprintf("%s/%s", archName, imageTypeName))
		}
	}

	expected := []string{
		"aarch64/qcow2",
		"fakearch/qcow2",
		"x86_64/qcow2",
	}

	assert.ElementsMatch(expected, archImageTypes)
}
