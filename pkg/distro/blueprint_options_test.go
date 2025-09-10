package distro_test

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/osbuild/blueprint/pkg/blueprint"
	"github.com/osbuild/images/pkg/distro"
	"github.com/osbuild/images/pkg/distrofactory"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBlueprintOptions(t *testing.T) {
	require := require.New(t)
	distroFactory := distrofactory.NewDefault()
	require.NotNil(distroFactory)

	distros := listTestedDistros(t)
	require.NotEmpty(distros)

	// TODO: iterate distros, arches, image types
	distroName := "fedora-43"
	fedora := distroFactory.GetDistro(distroName)
	require.NotNil(fedora)

	archName := "x86_64"
	x86, err := fedora.GetArch(archName)
	require.NoError(err)

	imageTypeName := "server-qcow2"
	qcow2, err := x86.GetImageType(imageTypeName)
	require.NoError(err)

	// TODO: iterate supported options
	require.NoError(checkSupportedOption(t, qcow2, "customizations.kernel"))

	// TODO: iterate unsupported options (somehow)
	require.NoError(checkUnsupportedOption(t, qcow2, "customizations.installer"))
}

func checkSupportedOption(t *testing.T, imageType distro.ImageType, optionPath string) error {
	var rngSeed int64 = 12039
	empty := &blueprint.Blueprint{}
	options := distro.ImageOptions{}
	emptyManifest, _, err := imageType.Manifest(empty, options, nil, &rngSeed)
	if err != nil {
		return err
	}

	testBP := &blueprint.Blueprint{}
	bpv := reflect.ValueOf(testBP)
	if err := fillValues(bpv, optionPath); err != nil {
		return err
	}
	bpManifest, _, err := imageType.Manifest(testBP, options, nil, &rngSeed)
	if err != nil {
		return err
	}

	assert.NotEqual(t, emptyManifest, bpManifest)
	return nil
}

func checkUnsupportedOption(t *testing.T, imageType distro.ImageType, optionPath string) error {
	var rngSeed int64 = 12039
	empty := &blueprint.Blueprint{}
	options := distro.ImageOptions{}
	emptyManifest, _, err := imageType.Manifest(empty, options, nil, &rngSeed)
	if err != nil {
		return err
	}

	testBP := &blueprint.Blueprint{}
	bpv := reflect.ValueOf(testBP)
	if err := fillValues(bpv, optionPath); err != nil {
		return err
	}
	bpManifest, _, err := imageType.Manifest(testBP, options, nil, &rngSeed)
	if err != nil {
		return err
	}

	assert.Equal(t, emptyManifest, bpManifest)
	return nil
}

func fillValues(v reflect.Value, path string) error {
	structType := v.Type()
	if v.Kind() == reflect.Pointer {
		// deref to get concrete type and element
		structType = structType.Elem()
		v = v.Elem()
	}

	pathComponents := strings.SplitN(path, ".", 2)
	curTag := pathComponents[0]
	restOfPath := ""
	if len(pathComponents) == 2 {
		restOfPath = pathComponents[1]
	}

	for idx := range structType.NumField() {
		structField := structType.Field(idx)
		fieldTag := distro.JSONTagFor(structField)
		// if the curTag is "", then we are in a substruct that is fully
		// supported, so we can fill in everything
		if fieldTag != curTag && curTag != "" {
			continue
		}

		newFieldValue := reflect.New(structField.Type).Elem()
		nv, err := setRandValue(newFieldValue, restOfPath)
		if err != nil {
			return err
		}
		newFieldValue = nv
		fieldValue := v.Field(idx)
		fieldValue.Set(newFieldValue)
	}

	return nil
}

func setRandValue(v reflect.Value, path string) (reflect.Value, error) {
	switch v.Kind() {
	case reflect.String:
		v.SetString("foo") // TODO: actual random value
	case reflect.Bool:
		v.SetBool(true) // NOTE: can't randomize because the default value is always false
	case reflect.Int:
		v.SetInt(13) // TODO: actual random value
	case reflect.Uint64:
		v.SetUint(13) // TODO: actual random value
	case reflect.Slice:
		elemType := v.Type().Elem()
		for range 3 { // TODO: random len
			// add 3 elements
			elemValue := reflect.New(elemType).Elem()
			newEv, err := setRandValue(elemValue, path)
			if err != nil {
				return v, err
			}
			elemValue = newEv
			v = reflect.Append(v, elemValue)
		}
	case reflect.Struct:
		// back to the top
		if err := fillValues(v, path); err != nil {
			return v, err
		}
	case reflect.Pointer:
		// deref and descend
		elemType := v.Type().Elem()

		cvp := reflect.New(elemType)
		cv := cvp.Elem()
		newCv, err := setRandValue(cv, path)
		if err != nil {
			return v, err
		}
		cv = newCv
		v = cvp
	default:
		fmt.Printf("Unhandled type: %s (%#v)\n", v.Type(), v)
		// return v, fmt.Errorf("Unhandled type: %s (%#v)\n", v.Type(), v)
	}
	return v, nil
}

func DontTestBlueprintOptions(t *testing.T) {
	assert := assert.New(t)
	distroFactory := distrofactory.NewDefault()
	assert.NotNil(distroFactory)

	distros := listTestedDistros(t)
	assert.NotEmpty(distros)

	for _, distroName := range distros {
		d := distroFactory.GetDistro(distroName)
		assert.NotNil(d)

		arches := d.ListArches()
		assert.NotEmpty(arches)

		for _, archName := range arches {
			arch, err := d.GetArch(archName)
			assert.Nil(err)

			imgTypes := arch.ListImageTypes()
			assert.NotEmpty(imgTypes)

			for _, imageTypeName := range imgTypes {
				t.Run(fmt.Sprintf("%s/%s/%s", distroName, archName, imageTypeName), func(t *testing.T) {
					if imageTypeName != "qcow2" {
						// XXX: remove after PoC
						return
					}

					t.Parallel()
					imageType, err := arch.GetImageType(imageTypeName)
					assert.Nil(err)

					supported := imageType.SupportedBlueprintOptions()

					for _, option := range supported {
						bp, err := fillBlueprintFields(option)
						assert.NoError(err)
						options := distro.ImageOptions{}
						_, _, err = imageType.Manifest(&bp, options, nil, nil)
						assert.NoError(err)
					}
				})
			}
		}
	}
}

func fillBlueprintFields(path string) (blueprint.Blueprint, error) {
	fmt.Printf("Searching for %q\n", path)
	bp := blueprint.Blueprint{}
	bpv := reflect.ValueOf(bp)
	// find the blueprint field represented by the option and set it
	if err := findBlueprintField(bpv, path); err != nil {
		return bp, err
	}
	return bp, nil
}

func findBlueprintField(v reflect.Value, path string) error {
	// parts := strings.SplitN(path, ".", 2)
	// if len(parts) == 1 {
	// 	// found the last component in the path
	// 	// print it and return
	// 	fmt.Printf("Found field at %q: %+v\n", path, v.Type())
	// 	return nil
	// }

	// field, err := distro.FieldByTag(v, parts[0])
	// if err != nil {
	// 	return err
	// }

	// // the path has more components, so we need to recurse deeper
	// findBlueprintField(field, parts[1])
	return nil
}
