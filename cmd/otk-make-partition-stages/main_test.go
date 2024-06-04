package main_test

import (
	"bytes"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	makestages "github.com/osbuild/images/cmd/otk-make-partition-stages"
	"github.com/osbuild/images/pkg/disk"
)

func TestIntegration(t *testing.T) {
	// this is not symetrical to the output, this is sad but also
	// okay because the input is really just a dump of the internal
	// disk.PartitionTable so encoding it in json here will not add
	// a benefit for the test
	minimalInput := makestages.Input{
		Internal: makestages.InputInternal{
			PartitionTable: &disk.PartitionTable{
				Size: 10738466816,
				UUID: "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
				Type: "dos",
				Partitions: []disk.Partition{
					{
						Start: 1048576,
						Size:  10737418240,
						Payload: &disk.Filesystem{
							Type:       "ext4",
							UUID:       "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75",
							Mountpoint: "/",
						},
					},
				},
			},
		},
	}
	expectedStages := `[
  {
    "type": "org.osbuild.truncate",
    "options": {
      "filename": "disk.img",
      "size": "10738466816"
    }
  },
  {
    "type": "org.osbuild.sfdisk",
    "options": {
      "label": "dos",
      "uuid": "0194fdc2-fa2f-4cc0-81d3-ff12045b73c8",
      "partitions": [
        {
          "size": 20971520,
          "start": 2048
        }
      ]
    },
    "devices": {
      "device": {
        "type": "org.osbuild.loopback",
        "options": {
          "filename": "disk.img",
          "lock": true
        }
      }
    }
  },
  {
    "type": "org.osbuild.mkfs.ext4",
    "options": {
      "uuid": "6e4ff95f-f662-45ee-a82a-bdf44a2d0b75"
    },
    "devices": {
      "device": {
        "type": "org.osbuild.loopback",
        "options": {
          "filename": "disk.img",
          "start": 2048,
          "size": 20971520,
          "lock": true
        }
      }
    }
  }
]
`
	inpJSON, err := json.Marshal(&minimalInput)
	assert.NoError(t, err)
	fakeStdin := bytes.NewBuffer(inpJSON)
	fakeStdout := bytes.NewBuffer(nil)

	err = makestages.Run(fakeStdin, fakeStdout)
	assert.NoError(t, err)

	assert.Equal(t, expectedStages, fakeStdout.String())
}
