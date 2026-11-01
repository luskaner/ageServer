package config

import (
	"runtime"

	"github.com/spf13/pflag"
)

type RevertMinimalValues struct {
	IPs   bool
	Certs bool
}

func (v *RevertMinimalValues) IPsRef() *bool {
	return &v.IPs
}

func (v *RevertMinimalValues) CertsRef() *bool {
	return &v.Certs
}

func NewRevertMinimalValues() *RevertMinimalValues {
	return &RevertMinimalValues{}
}

type RevertBaseValues struct {
	*RevertMinimalValues
	RemoveAll bool
}

func (v *RevertBaseValues) RemoveAllRef() *bool {
	return &v.RemoveAll
}

func InitBaseRevert(flags *pflag.FlagSet) (values *RevertBaseValues) {
	values = &RevertBaseValues{RevertMinimalValues: NewRevertMinimalValues()}
	flags.BoolVarP(
		values.IPsRef(),
		"ip",
		"i",
		false,
		"Remove the IP mappings from the local DNS server",
	)
	flags.BoolVarP(
		values.CertsRef(),
		"localCert",
		"l",
		false,
		"Remove the certificate from the local machine's trusted root store",
	)
	flags.BoolVarP(
		values.RemoveAllRef(),
		"all",
		"a",
		false,
		"Removes all configuration. Equivalent to the rest of the flags being set without fail-fast.",
	)
	return values
}

type RevertValues struct {
	*CommonBaseValues
	*RevertBaseValues
	RemoveUserCert     bool
	RestoreCAStoreCert bool
}

func NewRevertValues() RevertValues {
	return RevertValues{
		CommonBaseValues: NewCommonBaseValues(),
		RevertBaseValues: &RevertBaseValues{},
	}
}

func (v *RevertValues) RemoveUserCertRef() *bool {
	return &v.RemoveUserCert
}

func (v *RevertValues) RestoreCAStoreCertRef() *bool {
	return &v.RestoreCAStoreCert
}

func RevertFlagSet() (values *RevertValues, flags *pflag.FlagSet) {
	values = new(NewRevertValues())
	flags = pflag.NewFlagSet("revert", pflag.ContinueOnError)
	values.RevertBaseValues = InitBaseRevert(flags)
	values.CommonBaseValues = AddCommonFlags(
		flags,
		"",
		"",
		"",
		"Restore metadata. Unnecessary for AoE:DE",
		"Restore profiles.",
	)
	if runtime.GOOS != "linux" {
		flags.BoolVarP(values.RemoveUserCertRef(), "userCert", "u", false, "Remove the certificate from the user's trusted root store")
	}
	flags.BoolVarP(values.RestoreCAStoreCertRef(), "caStoreCert", "s", false, "Restore the game's trusted root store. For all except AoE I: DE and AoE IV: AE.")
	return
}
