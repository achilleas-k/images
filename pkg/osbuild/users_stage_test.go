package osbuild

import (
	"crypto/sha256"
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/osbuild/images/internal/common"
	"github.com/osbuild/images/pkg/customizations/users"
)

func TestNewUsersStage(t *testing.T) {
	expectedStage := &Stage{
		Type:    "org.osbuild.users",
		Options: &UsersStageOptions{},
	}
	actualStage := NewUsersStage(&UsersStageOptions{})
	assert.Equal(t, expectedStage, actualStage)
}

func TestNewUsersStageOptionsPassword(t *testing.T) {
	Pass := "testpass"
	EmptyPass := ""
	CryptPass := "$6$RWdHzrPfoM6BMuIP$gKYlBXQuJgP.G2j2twbOyxYjFDPUQw8Jp.gWe1WD/obX0RMyfgw5vt.Mn/tLLX4mQjaklSiIzoAW3HrVQRg4Q." // #nosec G101

	users := []users.User{
		{
			Name:     "bart",
			Password: &Pass,
		},
		{
			Name:     "lisa",
			Password: &CryptPass,
		},
		{
			Name:     "maggie",
			Password: &EmptyPass,
		},
		{
			Name: "homer",
		},
	}

	options, err := NewUsersStageOptions(users, false)
	require.Nil(t, err)
	require.NotNil(t, options)

	// bart's password should now be a hash
	assert.True(t, strings.HasPrefix(*options.Users["bart"].Password, "$6$"))

	// lisa's password should be left alone (already hashed)
	assert.Equal(t, CryptPass, *options.Users["lisa"].Password)

	// maggie's password should now be nil (locked account)
	assert.Nil(t, options.Users["maggie"].Password)

	// homer's password should still be nil (locked account)
	assert.Nil(t, options.Users["homer"].Password)
}

func TestGenUsersStageSameAsNewUsersStageOptions(t *testing.T) {
	users := []users.User{
		{
			Name: "user1", UID: common.ToPtr(1000), GID: common.ToPtr(1000),
			Groups:      []string{"grp1"},
			Description: common.ToPtr("some-descr"),
			Home:        common.ToPtr("/home/user1"),
			Shell:       common.ToPtr("/bin/zsh"),
			Key:         common.ToPtr("some-key"),
		},
	}
	expected := &UsersStageOptions{
		Users: map[string]UsersStageOptionsUser{
			"user1": {
				UID:         common.ToPtr(1000),
				GID:         common.ToPtr(1000),
				Groups:      []string{"grp1"},
				Description: common.ToPtr("some-descr"),
				Home:        common.ToPtr("/home/user1"),
				Shell:       common.ToPtr("/bin/zsh"),
				Key:         common.ToPtr("some-key")},
		},
	}

	// check that NewUsersStageOptions creates the expected user options
	opts, err := NewUsersStageOptions(users, false)
	require.Nil(t, err)
	assert.Equal(t, opts, expected)

	// check that GenUsersStage creates the expected user options
	st, err := GenUsersStage(users, false)
	require.Nil(t, err)
	usrStageOptions := st.Options.(*UsersStageOptions)
	assert.Equal(t, usrStageOptions, expected)

	// and (for good measure, not really needed) check that both gen
	// the same
	assert.Equal(t, usrStageOptions, opts)
}

func TestGenSudoersFilesStages(t *testing.T) {
	type testCase struct {
		users  []users.User
		stages []*Stage
		expErr error
	}

	adminFileSum := sha256.Sum256([]byte("admin\tALL=(ALL)\tNOPASSWD: ALL"))

	testCases := map[string]testCase{
		"happy1": testCase{
			users: []users.User{
				{
					Name:         "admin",
					SudoNopasswd: common.ToPtr(true),
				},
				{
					Name:         "notadmin",
					SudoNopasswd: common.ToPtr(false),
				},
				{
					Name: "some-user",
				},
			},
			stages: []*Stage{
				{
					Type: "org.osbuild.copy",
					Options: &CopyStageOptions{
						Paths: []CopyStagePath{
							{
								From:              fmt.Sprintf("input://file-%[1]x/sha256:%[1]x", adminFileSum),
								To:                "tree:///etc/sudoers.d/admin",
								RemoveDestination: true,
							},
						},
					},
					Inputs: &CopyStageFilesInputs{
						fmt.Sprintf("file-%x", adminFileSum): {
							inputCommon: inputCommon{
								Type:   "org.osbuild.files",
								Origin: "org.osbuild.source",
							},
							References: &FilesInputSourceArrayRef{
								{
									ID: fmt.Sprintf("sha256:%x", adminFileSum),
								},
							},
						},
					},
				},
				{
					Type: "org.osbuild.chown",
					Options: &ChownStageOptions{
						Items: map[string]ChownStagePathOptions{
							"/etc/sudoers.d/admin": {
								User:      "root",
								Group:     "root",
								Recursive: false,
							},
						},
					},
				},
			},
		},
	}

	for name, tc := range testCases {
		t.Run(name, func(t *testing.T) {
			assert := assert.New(t)
			stages, err := GenSudoersFilesStages(tc.users)
			if tc.expErr != nil {
				assert.Equal(tc.expErr, err)
			} else {
				assert.NoError(err)
				assert.Equal(tc.stages, stages)
			}
		})
	}
}
