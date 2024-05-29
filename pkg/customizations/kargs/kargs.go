package kargs

import (
	"fmt"
	"strings"

	"github.com/osbuild/images/internal/common"
	"golang.org/x/exp/slices"
)

type RootPerms string

const (
	RootPermsUnset RootPerms = ""
	RootPermsRW    RootPerms = "rw"
	RootPermsRO    RootPerms = "ro"
)

type Options struct {

	// Consistent network device naming using biosdevname.
	// Note that unless the system is a Dell system, or biosdevname is
	// explicitly enabled with this option, the systemd naming scheme will take
	// precedence.
	// If the biosdevname install option is specified, it must remain as a boot
	// option for the lifetime of the system.
	Biosdevname *bool

	// Specifies devices for console output. Multiple options can be specified.
	// Output will appear on all of them. The last device will be used when
	// opening /dev/console.
	Console []string

	// Using kexec, Linux can switch to a 'crash kernel' upon panic. This
	// parameter reserves the physical memory region for that kernel image.
	Crashkernel *string

	// All Kernel messages with a loglevel smaller than the console loglevel
	// will be printed to the console.
	Loglevel *uint

	// The modprobe.blacklist option will prevent the automatic loading of the
	// module by the kernel (however, manual loading is still possible).
	ModprobeBlacklist []string

	// Network interfaces are renamed to give them predictable names when
	// possible. It is enabled by default.
	NetIfnames *bool

	// Disables the code which tests for broken timer IRQ sources.
	NoTimerCheck *bool

	// The 'ro' option tells the kernel to mount the root filesystem as
	// 'read-only' so that filesystem consistency check programs (fsck) can do
	// their work on a quiescent filesystem. No processes can write to files
	// on the filesystem in question until it is 'remounted' as read/write
	// capable, for example, by 'mount -w -n -o remount /'.
	// The 'rw' option tells the kernel to mount the root filesystem
	// read/write.  This is the default.
	RootPerms RootPerms

	// Add extra options not handled by this package. These will be appended to
	// the kernel arguments directly, separated by spaces.
	Extra []string
}

func boolPtrToString(b *bool) string {
	if b == nil {
		return ""
	}
	if *b {
		return "1"
	}
	return "0"
}

func (o Options) StringList() []string {
	optionsStr := []string{}

	if biosdevname := boolPtrToString(o.Biosdevname); biosdevname != "" {
		optionsStr = append(optionsStr, fmt.Sprintf("biosdevname=%s", biosdevname))
	}

	for _, consoleOpt := range o.Console {
		optionsStr = append(optionsStr, fmt.Sprintf("console=%s", consoleOpt))
	}

	if o.Crashkernel != nil {
		optionsStr = append(optionsStr, fmt.Sprintf("crashkernel=%s", *o.Crashkernel))
	}

	if o.Loglevel != nil {
		optionsStr = append(optionsStr, fmt.Sprintf("loglevel=%d", *o.Loglevel))
	}

	if len(o.ModprobeBlacklist) > 0 {
		optionsStr = append(optionsStr, fmt.Sprintf("modprobe.blacklist=%s", strings.Join(o.ModprobeBlacklist, ",")))
	}

	if netifnames := boolPtrToString(o.NetIfnames); netifnames != "" {
		optionsStr = append(optionsStr, fmt.Sprintf("net.ifnames=%s", netifnames))
	}

	if noTimerCheck := boolPtrToString(o.NoTimerCheck); noTimerCheck == "1" {
		optionsStr = append(optionsStr, "no_timer_check")
	}

	if o.RootPerms != RootPermsUnset {
		optionsStr = append(optionsStr, string(o.RootPerms))
	}

	optionsStr = append(optionsStr, o.Extra...)

	return optionsStr
}

func (o Options) String() string {
	return strings.Join(o.StringList(), " ")
}

// Update properties in the receiver with values from the argument. Properties
// with nil or empty values in the argument do not affect the properties in the
// receiver. String slices (Console, ModprobeBlacklist, Extra) are
// concatenated.
func (o *Options) Update(newOpts *Options) {
	if o == nil {
		o = &Options{}
	}

	if newOpts.Biosdevname != nil {
		o.Biosdevname = newOpts.Biosdevname
	}

	o.Console = append(o.Console, newOpts.Console...)

	if newOpts.Crashkernel != nil {
		o.Crashkernel = newOpts.Crashkernel
	}

	if newOpts.Loglevel != nil {
		o.Loglevel = newOpts.Loglevel
	}

	o.ModprobeBlacklist = append(o.ModprobeBlacklist, newOpts.ModprobeBlacklist...)

	if newOpts.NetIfnames != nil {
		o.NetIfnames = newOpts.NetIfnames
	}

	if newOpts.NoTimerCheck != nil {
		o.NoTimerCheck = newOpts.NoTimerCheck
	}

	if newOpts.RootPerms != RootPermsUnset {
		o.RootPerms = newOpts.RootPerms
	}

	o.Extra = append(o.Extra, newOpts.Extra...)
}

func (o Options) Copy() Options {
	return Options{
		Biosdevname:       common.PtrValueCopy(o.Biosdevname),
		Console:           slices.Clone(o.Console),
		Crashkernel:       common.PtrValueCopy(o.Crashkernel),
		Loglevel:          common.PtrValueCopy(o.Loglevel),
		ModprobeBlacklist: slices.Clone(o.ModprobeBlacklist),
		NetIfnames:        common.PtrValueCopy(o.NetIfnames),
		NoTimerCheck:      o.NoTimerCheck,
		RootPerms:         o.RootPerms,
		Extra:             slices.Clone(o.Extra),
	}
}
