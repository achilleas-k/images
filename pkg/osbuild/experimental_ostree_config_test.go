package osbuild

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewExperimentalOSTreeConfigStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.experimental.ostree.config",
		Options: &ExperimentalOSTreeConfigStageOptions{},
	}
	actualStage := NewExperimentalOSTreeConfigStage(&ExperimentalOSTreeConfigStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}
func TestExperimentalOSTreeConfigStageValidate(t *testing.T) {

	type testCase struct {
		options ExperimentalOSTreeConfigStageOptions
		errmsg  string
	}

	testCases := map[string]testCase{
		"happy-repo-only": {
			options: ExperimentalOSTreeConfigStageOptions{
				Repo: "/tmp/whatever",
			},
			errmsg: "",
		},
		"happy-compose-empty": {
			options: ExperimentalOSTreeConfigStageOptions{
				Repo: "/tmp/whatever",
				Config: &ExperimentalOSTreeConfig{
					Integrity: &ExperimentalOSTreeConfigIntegrity{
						Composefs: "",
					},
				},
			},
			errmsg: "",
		},
		"happy-compose-true": {
			options: ExperimentalOSTreeConfigStageOptions{
				Repo: "/tmp/whatever",
				Config: &ExperimentalOSTreeConfig{
					Integrity: &ExperimentalOSTreeConfigIntegrity{
						Composefs: "true",
					},
				},
			},
			errmsg: "",
		},
		"happy-compose-false": {
			options: ExperimentalOSTreeConfigStageOptions{
				Repo: "/tmp/whatever",
				Config: &ExperimentalOSTreeConfig{
					Integrity: &ExperimentalOSTreeConfigIntegrity{
						Composefs: "false",
					},
				},
			},
			errmsg: "",
		},
		"happy-compose-maybe": {
			options: ExperimentalOSTreeConfigStageOptions{
				Repo: "/tmp/whatever",
				Config: &ExperimentalOSTreeConfig{
					Integrity: &ExperimentalOSTreeConfigIntegrity{
						Composefs: "maybe",
					},
				},
			},
			errmsg: "",
		},
		"unhappy-norepo": {
			options: ExperimentalOSTreeConfigStageOptions{},
			errmsg:  "experimental ostree config requires repo",
		},
		"unhappy-val": {
			options: ExperimentalOSTreeConfigStageOptions{
				Repo: "/tmp/whatever",
				Config: &ExperimentalOSTreeConfig{
					Integrity: &ExperimentalOSTreeConfigIntegrity{
						Composefs: "42",
					},
				},
			},
			errmsg: "experimental ostree config.integrity.composefs must be one of true, false, maybe",
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			err := tc.options.validate()
			switch tc.errmsg {
			case "":
				assert.Nil(t, err)
			default:
				require.NotNil(t, err)
				assert.Equal(t, err.Error(), tc.errmsg)
			}

		},
		)
	}
}
