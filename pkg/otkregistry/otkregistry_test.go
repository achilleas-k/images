package otkregistry_test

import (
	"testing"

	"github.com/osbuild/images/pkg/otkregistry"
	"github.com/stretchr/testify/require"
)

func TestRegistryLoad(t *testing.T) {
	require := require.New(t)

	// TODO: we can write the fragments during the test setup and make the
	// whole test self-contained
	registry, err := otkregistry.New("../../test/data/otk")
	require.NoError(err)
	require.Len(registry.Distributions, 1)

	distro := registry.Distributions[0]
	require.Equal("FakeDistro", distro.Name())
}
