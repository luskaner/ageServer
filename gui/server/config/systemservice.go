package main

// SystemService reports how the desktop the application runs on looks, so the
// interface can follow it instead of imposing colours of its own.
type SystemService struct{}

// AccentColour returns the colour the desktop highlights controls with, as a
// #rrggbb string. It is empty when the system offers no accent colour to read,
// and the interface then keeps its own.
func (s *SystemService) AccentColour() string {
	return systemAccentColour()
}
