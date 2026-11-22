package config

import (
	"errors"
	"net"
	"runtime"

	"github.com/spf13/pflag"
)

// ValidateSetupHostSet rejects a run asked to map the API domains and the rest
// of them at the same time: they are the two halves of the game's domains, so
// both set means the same as neither. Everything else builds on AllHosts
// staying the union of the two.
func ValidateSetupHostSet(v *SetupBaseValues) error {
	if v.Api && v.RootOnly {
		return errors.New("--api and --rootOnly are mutually exclusive")
	}
	return nil
}

type SetupBaseValues struct {
	MapIp                  net.IP
	MacOsExclusiveMappings bool
	CanUseInternet         bool
	AddLocalCertData       []byte
	// Api maps only the API domains, so a setup run can point them at a
	// standalone API server while another run maps the rest at the server.
	Api bool
	// RootOnly maps every domain but the API ones, the counterpart pass when
	// a previous (or the next) run mapped the API domains separately. Api and
	// RootOnly are mutually exclusive: a run maps one of the two halves, or
	// everything when neither is set.
	RootOnly bool
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
	AddSigKeyData   []byte
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

func (v *SetupValues) AddSigKeyDataRef() *[]byte {
	return &v.AddSigKeyData
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
	flags.BoolVar(&values.Api, "api", false, "Map only the API domains, leaving the rest of the game's domains alone.")
	flags.BoolVar(&values.RootOnly, "rootOnly", false, "Map every domain but the API ones, the server's half of a run whose API domains are mapped separately.")
	_ = flags.MarkHidden("rootOnly")
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
		"It requires the 'sigKeyData' option to be set. If non-empty the signing key will be saved only to the specified path.",
		"Backup metadata. Unnecessary for AoE:DE",
		"Backup profiles.",
	)
	if runtime.GOOS != "linux" {
		flags.BytesBase64VarP(values.AddUserCertDataRef(), "userCert", "u", nil, "Add the certificate to the user's trusted root store")
	}
	flags.BytesBase64VarP(values.AddCACertDataRef(), "caStoreCert", "s", nil, "Add the certificate to the game's trusted root store. For all except AoE I: DE and AoE IV: AE.")
	flags.BytesBase64Var(values.AddSigKeyDataRef(), "sigKeyData", nil, "The server's public signing key (DER) to save to 'sigKeyFilePath'.")
	_ = flags.MarkHidden("sigKeyData")
	flags.BoolVarP(&values.AgentEndOnError, "agentEndOnError", "r", false, "Stop the 'config-admin-agent' if it is running and any admin action failed.")
	_ = flags.MarkHidden("agentEndOnError")
	return
}
