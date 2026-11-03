package hosts

import (
	"fmt"
	"net"
	"slices"
	"strings"

	"github.com/luskaner/ageServer/common"
)

const (
	// marking labels lines written by AgeServer that are active mappings.
	marking = common.Name
	// preservedMarking labels commented copies of user lines that AgeServer
	// preserves so a revert can restore them. It is distinct from marking so a
	// revert never uncomments a legacy owned comment by accident.
	preservedMarking = marking + "-preserved"
)

type Line struct {
	ip       net.IP
	hosts    []Host
	comments []string
}

func (l Line) IP() net.IP {
	return l.ip
}

func (l Line) Hosts() []Host {
	return l.hosts
}

func (l Line) Own() bool {
	if len(l.comments) == 0 {
		return false
	}
	last := l.comments[len(l.comments)-1]
	return last == marking || last == preservedMarking
}

// Preserved reports whether the line is a commented copy of a user line that
// AgeServer should restore on revert.
func (l Line) Preserved() bool {
	if len(l.comments) == 0 {
		return false
	}
	return l.comments[len(l.comments)-1] == preservedMarking
}

func (l Line) OnlyComments() bool {
	return l.ip == nil && len(l.hosts) == 0
}

func (l Line) String() string {
	sb := strings.Builder{}
	if l.ip != nil {
		sb.WriteString(l.ip.String())
		sb.WriteString("\t")
		hostsStr := make([]string, len(l.hosts))
		for i, host := range l.hosts {
			hostsStr[i] = host.String()
		}
		sb.WriteString(strings.Join(hostsStr, ` `))
	}
	if len(l.comments) > 0 {
		if sb.Len() > 0 {
			sb.WriteByte(' ')
		}
		sb.WriteRune(commentMarker)
		sb.WriteString(strings.Join(l.comments, string(commentMarker)))
	}
	return sb.String()
}

func (l Line) Commented() (ok bool, nl Line) {
	if l.OnlyComments() {
		return true, l
	}
	commented := fmt.Sprintf("%c%s", commentMarker, l.String())
	ok, _, nl = ParseLine(commented, true)
	return
}

func (l Line) Uncommented() (nl string) {
	if !l.OnlyComments() {
		return l.String()
	}
	if len(l.comments) == 0 {
		return ""
	}
	sb := strings.Builder{}
	sb.WriteString(l.comments[0])
	if len(l.comments) > 1 {
		sb.WriteRune(commentMarker)
		sb.WriteString(strings.Join(l.comments[1:], string(commentMarker)))
	}
	return sb.String()
}

func (l Line) hasOwnMarking() bool {
	return l.Own()
}

func (l Line) WithoutOwnMarking() Line {
	if !l.hasOwnMarking() {
		return l
	}
	l.comments = slices.Clone(l.comments[:len(l.comments)-1])
	return l
}

func (l Line) WithOwnMarking() Line {
	if l.hasOwnMarking() {
		return l
	}
	l.comments = slices.Clone(append(l.comments, marking))
	return l
}

// WithPreservedMarking labels the line as a commented copy of a user line.
// Any previous own marking is replaced, so the line comes out as exactly one
// preserved marking.
func (l Line) WithPreservedMarking() Line {
	l = l.WithoutOwnMarking()
	l.comments = slices.Clone(append(l.comments, preservedMarking))
	return l
}
