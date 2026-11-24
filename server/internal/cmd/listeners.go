package cmd

import (
	"errors"
	"fmt"
	"net/netip"
	"slices"
	"strings"

	mapset "github.com/deckarep/golang-set/v3"
	"github.com/luskaner/ageServer/common"
	"github.com/luskaner/ageServer/server/internal/ip"
)

// listenerSpec is one address the process binds, with the certificate mode it
// presents. Every listener serves every enabled game; the mode only changes the
// certificate.
type listenerSpec struct {
	Addr netip.Addr
	Mode common.CertMode
}

func (l listenerSpec) String() string {
	return l.Addr.String() + ":443"
}

func resolveConfiguredHosts(hosts []string, listName string) (mapset.Set[netip.Addr], error) {
	addrs, resolveErrs := ip.ResolveHosts(mapset.NewThreadUnsafeSet[string](hosts...))
	errs := make([]error, 0, len(resolveErrs))
	for i := range resolveErrs {
		resolveErrs[i].List = listName
		errs = append(errs, &resolveErrs[i])
	}
	if len(errs) > 0 {
		return nil, errors.Join(errs...)
	}
	return addrs, nil
}

// validateListenerSets rejects the configurations whose binds would collide or
// whose meaning is ambiguous. Two non-empty sets are valid as long as they do
// not intersect and neither covers every interface.
func validateListenerSets(modern, legacy mapset.Set[netip.Addr]) error {
	if intersection := modern.Intersect(legacy); !intersection.IsEmpty() {
		return fmt.Errorf("Hosts.Modern and Hosts.Legacy resolve to the same address: %s", joinAddrs(intersection))
	}
	unspecified := netip.IPv4Unspecified()
	if modern.Contains(unspecified) && !legacy.IsEmpty() {
		return fmt.Errorf("Hosts.Modern binds 0.0.0.0, so Hosts.Legacy cannot bind any address")
	}
	if legacy.Contains(unspecified) && !modern.IsEmpty() {
		return fmt.Errorf("Hosts.Legacy binds 0.0.0.0, so Hosts.Modern cannot bind any address")
	}
	if modern.Contains(unspecified) && modern.Cardinality() > 1 {
		return fmt.Errorf("Hosts.Modern cannot combine 0.0.0.0 with other addresses")
	}
	if legacy.Contains(unspecified) && legacy.Cardinality() > 1 {
		return fmt.Errorf("Hosts.Legacy cannot combine 0.0.0.0 with other addresses")
	}
	return nil
}

func joinAddrs(addrs mapset.Set[netip.Addr]) string {
	list := addrs.ToSlice()
	slices.SortFunc(list, func(a, b netip.Addr) int { return a.Compare(b) })
	strs := make([]string, len(list))
	for i, addr := range list {
		strs[i] = addr.String()
	}
	return strings.Join(strs, ", ")
}

func listenerSpecs(modern, legacy mapset.Set[netip.Addr]) []listenerSpec {
	var specs []listenerSpec
	for addr := range modern.Iter() {
		specs = append(specs, listenerSpec{Addr: addr, Mode: common.CertModeModern})
	}
	for addr := range legacy.Iter() {
		specs = append(specs, listenerSpec{Addr: addr, Mode: common.CertModeLegacy})
	}
	slices.SortFunc(specs, func(a, b listenerSpec) int {
		if cmp := a.Addr.Compare(b.Addr); cmp != 0 {
			return cmp
		}
		return strings.Compare(string(a.Mode), string(b.Mode))
	})
	return specs
}

// buildListenerReport is the deterministic text printed at startup. It is a
// pure function so the golden test does not need to start a listener.
func buildListenerReport(specs []listenerSpec, gameIDs []string, filter common.CertMode) []string {
	var lines []string
	title := "Listener report:"
	if filter != "" {
		title = fmt.Sprintf("Listener report (listening only: %s):", filter)
	}
	lines = append(lines, title)
	ordered := slices.Clone(specs)
	slices.SortFunc(ordered, func(a, b listenerSpec) int {
		if cmp := a.Addr.Compare(b.Addr); cmp != 0 {
			return cmp
		}
		return strings.Compare(string(a.Mode), string(b.Mode))
	})
	for _, spec := range ordered {
		lines = append(lines, fmt.Sprintf("  %s  certificates: %s", spec, spec.Mode))
	}
	orderedGames := slices.Clone(gameIDs)
	slices.Sort(orderedGames)
	for _, gameID := range orderedGames {
		class := common.GameCertMode(gameID)
		lines = append(lines, "", fmt.Sprintf("Game %s (class %s):", gameID, class))
		found := false
		for _, spec := range ordered {
			if spec.Mode == class {
				lines = append(lines, fmt.Sprintf("  OK    %s (%s)", spec, spec.Mode))
				found = true
				continue
			}
			warning := "AoE IV / AoM may reject it"
			if class == common.CertModeLegacy {
				warning = "AoE I-III: DE may reject it (see M0)"
			}
			lines = append(lines, fmt.Sprintf("  WARN  %s (%s) — %s", spec, spec.Mode, warning))
		}
		if !found && len(ordered) == 0 {
			lines = append(lines, "  WARN  no listener")
		}
	}
	return lines
}
