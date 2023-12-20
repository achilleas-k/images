package cmdutil

import (
	"crypto/rand"
	"fmt"
	"math"
	"math/big"
	"os"
	"strconv"
)

// NewRNGSeed generates a random seed value unless the env var OSBUILD_RNG_SEED
// is set.
func NewRNGSeed() (int64, error) {
	envSeedStr := os.Getenv("OSBUILD_RNG_SEED")
	if envSeedStr != "" {
		envSeedInt, err := strconv.ParseInt(envSeedStr, 10, 64)
		if err != nil {
			return 0, fmt.Errorf("failed to parse OSBUILD_RNG_SEED value %q: %s", envSeedStr, err.Error())
		}
		fmt.Printf("TEST MODE: using rng seed %d\n", envSeedInt)
		return envSeedInt, nil
	}
	randSeed, err := rand.Int(rand.Reader, big.NewInt(math.MaxInt64))
	if err != nil {
		return 0, fmt.Errorf("failed to generate random seed: %s", err.Error())
	}
	return randSeed.Int64(), nil
}
