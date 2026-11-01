package dialog

import (
	"bufio"
	"fmt"
	"io"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/luskaner/ageServer/launcher-common/launcher"
	"github.com/luskaner/ageServer/launcher-common/ui"
)

// Nothing installed means the console: this frontend's whole point is that it
// runs where there is no window to ask in, so asking there is always possible,
// and it is also what the run falls back to when graphical dialogs are missing.
func init() { launcher.DefaultDialog = consoleDialog{} }

type consoleDialog struct{}

func (consoleDialog) Name() string { return "console" }

func (consoleDialog) SelectServer(servers []ServerCandidate, reader io.Reader) (int, bool) {
	count := len(servers)
	out := sinks()
	for {
		printCandidates(servers)
		out.Printf("%s Enter the number of the server (1-%d): ", ui.G(ui.GlyphQuestion), count)
		var option int
		if _, err := fmt.Fscan(reader, &option); err != nil {
			// Stdin exhausted or broken: we can never get a valid answer,
			// so retrying would spin forever printing the list.
			out.Println("Could not read selection from input.")
			return 0, false
		}
		if option < 1 || option > count {
			out.Println("Invalid option. Please enter a number from the list.")
			continue
		}
		return option - 1, true
	}
}

// ListCandidates prints the list exactly as SelectServer prints it before its
// prompt.
func (consoleDialog) ListCandidates(servers []ServerCandidate) { printCandidates(servers) }

func (consoleDialog) ConfirmStartServer(text string, reader io.Reader) bool {
	// An EOF is ignored on purpose: the prompt only ever offered "continue",
	// so there was never a way to say no and it must not abort the launch.
	sinks().Println(text + " Press enter to continue...")
	_, _ = bufio.NewReader(reader).ReadBytes('\n')
	return true
}

// printCandidates writes the numbered list of discovered servers to the
// console sinks. The console has room for the full description, so that is what
// it prints.
//
// The listing is left exactly as it has always been in the ASCII tier: a pipe, a
// redirected run and every test that asserts on this text keep seeing the same
// bytes. Only a console that can show box drawing gets the framed table.
func printCandidates(servers []ServerCandidate) {
	out := sinks()
	if ui.Current().Tier < ui.TierUnicode || len(servers) == 0 {
		out.Println("Found the following servers:")
		for i := range servers {
			out.Printf("%d. %s\n", i+1, servers[i].Description)
		}
		return
	}
	headers := []string{"#", "server"}
	rows := make([][]string, 0, len(servers))
	for i := range servers {
		rows = append(rows, []string{fmt.Sprintf("%d", i+1), servers[i].Description})
	}
	t := table.New().
		Border(lipgloss.RoundedBorder()).
		BorderStyle(lipgloss.NewStyle().Foreground(lipgloss.Color("240"))).
		StyleFunc(func(row, _ int) lipgloss.Style {
			if row == table.HeaderRow {
				return lipgloss.NewStyle().Bold(true)
			}
			return lipgloss.NewStyle()
		}).
		Headers(headers...).
		Rows(rows...)
	_, _ = fmt.Fprint(ui.Writer(), t.Render()+"\n")
}
