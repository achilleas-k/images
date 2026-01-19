package distro

// Tweaks is used to set small config changes and workarounds that are not
// specific image definition configurations but are required for builds to
// work.
type Tweaks struct {
	RPMKeys *RPMKeysTweaks `yaml:"rpmkeys"`
}

type RPMKeysTweaks struct {
	// UsePQRPM installs pqrpm in the build root in order to import PQC keys
	// and use them to verify packages for the OS and other non-build
	// pipelines.
	UsePQRPM bool `yaml:"use_pqrpm"`

	// IgnoreBuildImportFailures enables rpmkeys.ignore_import_failures for the
	// build pipeline's rpm stage. This is needed when building on a host
	// distro that does not support the format of one or more keys used by the
	// target distro's repository configs.
	IgnoreBuildImportFailures bool `yaml:"ignore_build_import_failures"`
}
