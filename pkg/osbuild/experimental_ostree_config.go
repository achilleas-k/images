package osbuild

import (
	"fmt"
	"strings"

	"golang.org/x/exp/slices"
)

// Options for the org.osbuild.experimental.ostree.config stage.
type ExperimentalOSTreeConfigStageOptions struct {
	Repo string `json:"repo"`

	Config *ExperimentalOSTreeConfig `json:"config,omitempty"`
}

func (ExperimentalOSTreeConfigStageOptions) isStageOptions() {}

type ExperimentalOSTreeConfig struct {
	Integrity *ExperimentalOSTreeConfigIntegrity `json:"integrity,omitempty"`
}

type ExperimentalOSTreeConfigIntegrity struct {
	Composefs string `json:"composefs,omitempty"`
}

func (o *ExperimentalOSTreeConfigStageOptions) validate() error {
	if o.Repo == "" {
		return fmt.Errorf("experimental ostree config requires repo")
	}

	if o.Config != nil &&
		o.Config.Integrity != nil &&
		o.Config.Integrity.Composefs != "" {

		validValues := []string{"true", "false", "maybe"}
		if !slices.Contains(validValues, o.Config.Integrity.Composefs) {
			return fmt.Errorf("experimental ostree config.integrity.composefs must be one of %s", strings.Join(validValues, ", "))
		}
	}
	return nil
}

// A new org.osbuild.experimental.ostree.config stage to configure experimental ostree options.
func NewExperimentalOSTreeConfigStage(options *ExperimentalOSTreeConfigStageOptions) *Stage {
	return &Stage{
		Type:    "org.osbuild.experimental.ostree.config",
		Options: options,
	}
}
