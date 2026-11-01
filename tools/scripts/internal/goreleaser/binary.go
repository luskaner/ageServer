package goreleaser

import mapset "github.com/deckarep/golang-set/v3"

type BinaryTargets map[OperatingSystem]map[Architecture]mapset.Set[string]

func NewBinaryTargets() *BinaryTargets {
	return new(make(BinaryTargets))
}

func (bt *BinaryTargets) Clone() *BinaryTargets {
	clone := NewBinaryTargets()
	clone.AddMultipleTargets(bt)
	return clone
}

func (bt *BinaryTargets) CloneForOperatingSystems(operatingSystems mapset.Set[OperatingSystem]) *BinaryTargets {
	clone := NewBinaryTargets()
	for os, currentTargets := range *bt {
		if operatingSystems.Contains(os) {
			for arch, instructionSets := range currentTargets {
				clone.AddTarget(os, arch, instructionSets.ToSlice()...)
			}
		}
	}
	return clone
}

func (bt *BinaryTargets) AddTarget(os OperatingSystem, arch Architecture, instructionSets ...string) {
	if !os.Archs().ContainsOne(arch) {
		panic("unsupported architecture for operating system")
	}
	instructionSetsSet := mapset.NewSet[string](instructionSets...)
	if !instructionSetsSet.IsSubset(arch.InstructionSet()) {
		panic("unsupported instruction set for architecture")
	}
	if _, ok := (*bt)[os]; !ok {
		(*bt)[os] = make(map[Architecture]mapset.Set[string])
	}
	if _, ok := (*bt)[os][arch]; !ok {
		(*bt)[os][arch] = mapset.NewSet[string]()
	}
	(*bt)[os][arch] = (*bt)[os][arch].Union(instructionSetsSet)
}

func (bt *BinaryTargets) AddMultipleTargets(multipleTargets ...*BinaryTargets) {
	for _, targets := range multipleTargets {
		for os, archs := range *targets {
			for arch, instructionSets := range archs {
				(*bt).AddTarget(os, arch, instructionSets.ToSlice()...)
			}
		}
	}
}

type Binary struct {
	targets *BinaryTargets
	main    string
}

func NewBinary(main string, targets *BinaryTargets) *Binary {
	return &Binary{
		main:    main,
		targets: targets,
	}
}

func (b *Binary) Main() string {
	return b.main
}

func (b *Binary) CloneForOperatingSystems(operatingSystems mapset.Set[OperatingSystem]) *Binary {
	return &Binary{
		main:    b.main,
		targets: b.targets.CloneForOperatingSystems(operatingSystems),
	}
}
