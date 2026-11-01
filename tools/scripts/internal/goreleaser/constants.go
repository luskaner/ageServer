package goreleaser

import (
	mapset "github.com/deckarep/golang-set/v3"
)

var UnixBasedOperatingSystems = mapset.NewSet(OSLinux, OSMacOS)
var operatingSystems = UnixBasedOperatingSystems.Union(mapset.NewSet(OSWindowsLegacy, OSWindowsModern))
var Targets64 *BinaryTargets
var Targets32 *BinaryTargets
var Targets3264 *BinaryTargets
var Targets64Windows *BinaryTargets
var x64Architectures = []Architecture{ArchAmd64, ArchArm64}
var x86Architectures = []Architecture{Arch386, ArchArm32}

func init() {
	Targets32 = NewBinaryTargets()
	Targets64 = NewBinaryTargets()
	Targets64Windows = NewBinaryTargets()
	for os := range operatingSystems.Iter() {
		for _, arch := range x86Architectures {
			if os.Archs().ContainsOne(arch) {
				var instructionSets []string
				if arch == ArchArm32 {
					instructionSets = []string{"5", "6"}
				}
				Targets32.AddTarget(os, arch, instructionSets...)
			}
		}
		for _, arch := range x64Architectures {
			if os.Archs().ContainsOne(arch) {
				Targets64.AddTarget(os, arch)
				if os == OSWindowsModern {
					Targets64Windows.AddTarget(os, arch)
				}
			}
		}
	}
	Targets3264 = Targets32.Clone()
	Targets3264.AddMultipleTargets(Targets64)
}
