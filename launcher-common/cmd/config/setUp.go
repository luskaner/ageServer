package config

import (
	"net"
	"runtime"

	"github.com/spf13/pflag"
)

type SetupBaseValues struct {
	MapIp                  net.IP
	MacOsExclusiveMappings bool
	CanUseInternet         bool
	AddLocalCertData       []byte
}

func (v *SetupBaseValues) MapIpRef() *net.IP {
	return &v.MapIp
}

func (v *SetupBaseValues) AddLocalCertDataRef() *[]byte {
	return &v.AddLocalCertData
}

func (v *SetupBaseValues) MacOsExclusiveMappingsRef() *bool {
	return &v.MacOsExclusiveMappings
}

type SetupValues struct {
	*CommonBaseValues
	*SetupBaseValues
	AddUserCertData []byte
	AddCACertData   []byte
	AgentEndOnError bool
}

func newSetupValues() SetupValues {
	return SetupValues{
		CommonBaseValues: NewCommonBaseValues(),
		SetupBaseValues:  &SetupBaseValues{},
	}
}

func (v *SetupValues) AddUserCertDataRef() *[]byte {
	return &v.AddUserCertData
}

func (v *SetupValues) AddCACertDataRef() *[]byte {
	return &v.AddCACertData
}

func InitSetUp(flags *pflag.FlagSet) (values *SetupBaseValues) {
	values = &SetupBaseValues{}
	flags.IPVarP(
		values.MapIpRef(),
		"ip",
		"i",
		nil,
		"IP to resolve in local DNS server.",
	)
	flags.BytesBase64VarP(
		values.AddLocalCertDataRef(),
		"localCert",
		"l",
		nil,
		"Add the certificate to the local machine's trusted root store",
	)
	flags.BoolVar(values.MacOsExclusiveMappingsRef(), "macExclusiveDomain", false, "macOS exclusive domain")
	flags.BoolVar(&values.CanUseInternet, "internet", true, "Whether or not the process may use the internet to look up official domains.")
	_ = flags.MarkHidden("internet")
	return values
}

func SetUpFlagSet() (values *SetupValues, flags *pflag.FlagSet) {
	values = new(newSetupValues())
	flags = pflag.NewFlagSet("setup", pflag.ContinueOnError)
	values.SetupBaseValues = InitSetUp(flags)
	values.CommonBaseValues = AddCommonFlags(
		flags,
		"Only relevant when using 'ip' option. If empty, it will use the system path",
		"It requires the 'localCert' option to be set. If non-empty the certificate will be saved only to the specified path.",
		"Backup metadata. Unnecessary for AoE:DE",
		"Backup profiles.",
	)
	if runtime.GOOS != "linux" {
		flags.BytesBase64VarP(values.AddUserCertDataRef(), "userCert", "u", nil, "Add the certificate to the user's trusted root store")
	}
	flags.BytesBase64VarP(values.AddCACertDataRef(), "caStoreCert", "s", nil, "Add the certificate to the game's trusted root store. For all except AoE I: DE and AoE IV: AE.")
	flags.BoolVarP(&values.AgentEndOnError, "agentEndOnError", "r", false, "Stop the 'config-admin-agent' if it is running and any admin action failed.")
	_ = flags.MarkHidden("agentEndOnError")
	return
}
