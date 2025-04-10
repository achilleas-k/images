package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/osbuild/images/internal/target"
	"github.com/osbuild/images/pkg/upload/azure"
)

// exitCheck can be deferred from the top of command functions to exit with an
// error code after any other defers are run in the same scope.
func exitCheck(err error) {
	if err != nil {
		fmt.Fprint(os.Stderr, err.Error()+"\n")
		os.Exit(1)
	}
}

// createUserData creates cloud-init's user-data that contains user redhat with
// the specified public key
func createUserData(username, publicKeyFile string) (string, error) {
	publicKey, err := os.ReadFile(publicKeyFile)
	if err != nil {
		return "", err
	}

	userData := fmt.Sprintf(`#cloud-config
user: %s
ssh_authorized_keys:
  - %s
`, username, string(publicKey))

	return userData, nil
}

// resources created or allocated for an instance that can be cleaned up when
// tearing down.
type resources struct {
	AMI           *string `json:"ami,omitempty"`
	Snapshot      *string `json:"snapshot,omitempty"`
	SecurityGroup *string `json:"security-group,omitempty"`
	InstanceID    *string `json:"instance,omitempty"`
}

func run(c string, args ...string) ([]byte, []byte, error) {
	fmt.Printf("> %s %s\n", c, strings.Join(args, " "))
	cmd := exec.Command(c, args...)

	var cmdout, cmderr bytes.Buffer
	cmd.Stdout = &cmdout
	cmd.Stderr = &cmderr
	err := cmd.Run()

	// print any output even if the call failed
	stdout := cmdout.Bytes()
	if len(stdout) > 0 {
		fmt.Println(string(stdout))
	}

	stderr := cmderr.Bytes()
	if len(stderr) > 0 {
		fmt.Fprintf(os.Stderr, "%s\n", string(stderr))
	}
	return stdout, stderr, err
}

func getInstanceType(arch string) (string, error) {
	switch arch {
	case "x86_64":
		return "t3.small", nil
	case "aarch64":
		return "t4g.medium", nil
	default:
		return "", fmt.Errorf("getInstanceType(): unknown architecture %q", arch)
	}
}

func sshRun(ip, user, key, hostsfile string, command ...string) error {
	sshargs := []string{"-i", key, "-o", fmt.Sprintf("UserKnownHostsFile=%s", hostsfile), "-l", user, ip}
	sshargs = append(sshargs, command...)
	_, _, err := run("ssh", sshargs...)
	if err != nil {
		return err
	}
	return nil
}

func scpFile(ip, user, key, hostsfile, source, dest string) error {
	_, _, err := run("scp", "-i", key, "-o", fmt.Sprintf("UserKnownHostsFile=%s", hostsfile), "--", source, fmt.Sprintf("%s@%s:%s", user, ip, dest))
	if err != nil {
		return err
	}
	return nil
}

func keyscan(ip, filepath string) error {
	var keys []byte
	maxTries := 30 // wait for at least 5 mins
	var keyscanErr error
	for try := 0; try < maxTries; try++ {
		keys, _, keyscanErr = run("ssh-keyscan", ip)
		if keyscanErr == nil {
			break
		}
		time.Sleep(10 * time.Second)
	}
	if keyscanErr != nil {
		return keyscanErr
	}

	fmt.Printf("Creating known hosts file: %s\n", filepath)
	hostsFile, err := os.Create(filepath)
	if err != nil {
		return err
	}

	fmt.Printf("Writing to known hosts file: %s\n", filepath)
	if _, err := hostsFile.Write(keys); err != nil {
		return err
	}
	return nil
}

func newClientFromArgs(flags *pflag.FlagSet) (*azure.Client, error) {
	credsFile, err := flags.GetString("credentials-file")
	if err != nil {
		return nil, err
	}

	tenantID, err := flags.GetString("tenant-id")
	if err != nil {
		return nil, err
	}

	subscriptionID, err := flags.GetString("subscription-id")
	if err != nil {
		return nil, err
	}

	creds, err := azure.ParseAzureCredentialsFile(credsFile)
	if err != nil {
		return nil, err
	}
	if creds == nil {
		return nil, fmt.Errorf("failed to read credentials file (but no error was produced)")
	}

	return azure.NewClient(*creds, tenantID, subscriptionID)
}

// getOptionalStringFlag returns the value of a string flag if it's set, or nil
// if it's not set.
func getOptionalStringFlag(flags *pflag.FlagSet, name string) (*string, error) {
	value, err := flags.GetString(name)
	if err != nil {
		return nil, err
	}
	if value == "" {
		return nil, nil
	}
	return &value, nil
}

func doSetup(client *azure.Client, filename string, flags *pflag.FlagSet, res *resources) error {
	client, err := newClientFromArgs(flags)
	if err != nil {
		return err
	}

	location, err := flags.GetString("location")
	if err != nil {
		return err
	}

	storageAccountTag := azure.Tag{
		Name:  "imageBuilderStorageAccount",
		Value: fmt.Sprintf("location=%s", location),
	}

	resourceGroup, err := flags.GetString("resource-group")
	if err != nil {
		return err
	}

	imageName, err := flags.GetString("image-name")
	if err != nil {
		return err
	}

	ctx := context.Background()
	storageAccount, err := client.GetResourceNameByTag(
		ctx,
		resourceGroup,
		storageAccountTag,
	)
	if err != nil {
		return err
	}

	if storageAccount == "" {
		const storageAccountPrefix = "image-boot-test"
		storageAccount = azure.RandomStorageAccountName(storageAccountPrefix)
		err := client.CreateStorageAccount(
			ctx,
			resourceGroup,
			storageAccount,
			location,
			storageAccountTag,
		)
		if err != nil {
			return err
		}
	}

	storageAccessKey, err := client.GetStorageAccountKey(
		ctx,
		resourceGroup,
		storageAccount,
	)
	if err != nil {
		return err
	}

	azureStorageClient, err := azure.NewStorageClient(storageAccount, storageAccessKey)
	if err != nil {
		return err
	}

	storageContainer := fmt.Sprintf("image-boot-tests-%s", uuid.New().String())

	err = azureStorageClient.CreateStorageContainerIfNotExist(ctx, storageAccount, storageContainer)
	if err != nil {
		return err
	}

	// Azure cannot create an image from a blob without .vhd extension
	blobName := azure.EnsureVHDExtension(imageName)

	fmt.Printf("[Azure] Uploading the image")
	err = azureStorageClient.UploadPageBlob(
		azure.BlobMetadata{
			StorageAccount: storageAccount,
			ContainerName:  storageContainer,
			BlobName:       blobName,
		},
		filename,
		4, // parallel upload routines
	)
	if err != nil {
		return err
	}

	fmt.Printf("[Azure] 📝 Registering the image")
	err = client.RegisterImage(
		ctx,
		resourceGroup,
		storageAccount,
		storageContainer,
		blobName,
		imageName,
		location,
		target.HyperVGenV2,
	)
	if err != nil {
		return err
	}
	fmt.Printf("[Azure] 🎉 Image uploaded and registered!")
	return nil
}

func setup(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	filename := args[0]
	flags := cmd.Flags()

	client, err := newClientFromArgs(flags)
	if err != nil {
		fnerr = err
		return
	}

	// collect resources into res and write them out when the function returns
	resourcesFile, err := flags.GetString("resourcefile")
	if err != nil {
		fnerr = err
		return
	}
	res := &resources{}

	fnerr = doSetup(client, filename, flags, res)
	if fnerr != nil {
		fmt.Fprintf(os.Stderr, "setup() failed: %s\n", fnerr.Error())
		fmt.Fprint(os.Stderr, "tearing down resources\n")
		tderr := doTeardown(client, res)
		if tderr != nil {
			fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
		}
	}

	resdata, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		fnerr = fmt.Errorf("failed to marshal resources data: %s", err.Error())
		return
	}
	resfile, err := os.Create(resourcesFile)
	if err != nil {
		fnerr = fmt.Errorf("failed to create resources file: %s", err.Error())
		return
	}
	_, err = resfile.Write(resdata)
	if err != nil {
		fnerr = fmt.Errorf("failed to write resources file: %s", err.Error())
		return
	}
	fmt.Printf("IDs for any newly created resources are stored in %s. Use the teardown command to clean them up.\n", resourcesFile)
	if err = resfile.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "error closing resources file: %s\n", err.Error())
		fnerr = err
		return
	}
}

func doTeardown(client *azure.Client, res *resources) error {
	return nil
}

func teardown(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()

	flags := cmd.Flags()

	a, err := newClientFromArgs(flags)
	if err != nil {
		fnerr = err
		return
	}

	resourcesFile, err := flags.GetString("resourcefile")
	if err != nil {
		return
	}

	res := &resources{}
	resfile, err := os.Open(resourcesFile)
	if err != nil {
		fnerr = fmt.Errorf("failed to open resources file: %s", err.Error())
		return
	}
	resdata, err := io.ReadAll(resfile)
	if err != nil {
		fnerr = fmt.Errorf("failed to read resources file: %s", err.Error())
		return
	}
	if err := json.Unmarshal(resdata, res); err != nil {
		fnerr = fmt.Errorf("failed to unmarshal resources data: %s", err.Error())
		return
	}

	fnerr = doTeardown(a, res)
}

func doRunExec(client *azure.Client, command []string, flags *pflag.FlagSet, res *resources) error {
	privKey, err := flags.GetString("ssh-privkey")
	if err != nil {
		return err
	}

	username, err := flags.GetString("username")
	if err != nil {
		return err
	}

	tmpdir, err := os.MkdirTemp("", "boot-test-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpdir)

	hostsfile := filepath.Join(tmpdir, "known_hosts")
	// ip, err := client.GetInstanceAddress(res.InstanceID)
	// if err != nil {
	// 	return err
	// }
	ip := ""
	if err := keyscan(ip, hostsfile); err != nil {
		return err
	}

	// ssh into the remote machine and exit immediately to check connection
	if err := sshRun(ip, username, privKey, hostsfile, "exit"); err != nil {
		return err
	}

	isFile := func(path string) bool {
		fileInfo, err := os.Stat(path)
		if err != nil {
			// ignore error and assume it's not a path
			return false
		}

		// Check if it's a regular file
		return fileInfo.Mode().IsRegular()
	}

	// copy every argument that is a file to the remote host (basename only)
	// and construct remote command
	// NOTE: this wont work with directories or with multiple args in different
	// paths that share the same basename - it's very limited
	remoteCommand := make([]string, len(command))
	for idx := range command {
		arg := command[idx]
		if isFile(arg) {
			// scp the file and add it to the remote command by its base name
			remotePath := filepath.Base(arg)
			remoteCommand[idx] = remotePath
			if err := scpFile(ip, username, privKey, hostsfile, arg, remotePath); err != nil {
				return err
			}
		} else {
			// not a file: add the arg as is
			remoteCommand[idx] = arg
		}
	}

	// add ./ to first element for the executable
	remoteCommand[0] = fmt.Sprintf("./%s", remoteCommand[0])

	// run the executable
	return sshRun(ip, username, privKey, hostsfile, remoteCommand...)
}

func runExec(cmd *cobra.Command, args []string) {
	var fnerr error
	defer func() { exitCheck(fnerr) }()
	image := args[0]

	command := args[1:]
	flags := cmd.Flags()

	a, fnerr := newClientFromArgs(flags)
	if fnerr != nil {
		return
	}

	res := &resources{}
	defer func() {
		tderr := doTeardown(a, res)
		if tderr != nil {
			// report it but let the exitCheck() handle fnerr
			fmt.Fprintf(os.Stderr, "teardown(): %s\n", tderr.Error())
		}
	}()

	fnerr = doSetup(a, image, flags, res)
	if fnerr != nil {
		return
	}

	fnerr = doRunExec(a, command, flags, res)
}

func setupCLI() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:                   "boot",
		Long:                  "upload and boot an image to the appropriate cloud provider",
		DisableFlagsInUseLine: true,
	}

	rootFlags := rootCmd.PersistentFlags()
	rootFlags.String("credentials-file", "", "path to file with Azure credentials")
	rootFlags.String("tenant-id", "", "")
	rootFlags.String("subscription-id", "", "")
	rootFlags.String("location", "", "")
	rootFlags.String("resource-group", "", "")

	// TODO: make it optional and use UUID if not specified
	rootFlags.String("image-name", "", "")

	setupCmd := &cobra.Command{
		Use:                   "setup [--resourcefile <filename>] <filename>",
		Short:                 "upload and boot an image and save the created resource IDs to a file for later teardown",
		Args:                  cobra.ExactArgs(1),
		Run:                   setup,
		DisableFlagsInUseLine: true,
	}
	setupCmd.Flags().StringP("resourcefile", "r", "resources.json", "path to store the resource IDs")
	rootCmd.AddCommand(setupCmd)

	teardownCmd := &cobra.Command{
		Use:   "teardown [--resourcefile <filename>]",
		Short: "teardown (clean up) all the resources specified in a resources file created by a previous 'setup' call",
		Args:  cobra.NoArgs,
		Run:   teardown,
	}
	teardownCmd.Flags().StringP("resourcefile", "r", "resources.json", "path to store the resource IDs")
	rootCmd.AddCommand(teardownCmd)

	runCmd := &cobra.Command{
		Use:   "run <image> <executable>...",
		Short: "upload and boot an image, then upload the specified executable and run it on the remote host",
		Long:  "upload and boot an image on AWS EC2, then upload the executable file specified by the second positional argument and execute it via SSH with the args on the command line",
		Args:  cobra.MinimumNArgs(2),
		Run:   runExec,
	}
	rootCmd.AddCommand(runCmd)

	return rootCmd
}

func main() {
	cmd := setupCLI()
	exitCheck(cmd.Execute())
}
