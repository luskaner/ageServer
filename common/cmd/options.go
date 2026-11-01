package cmd

import (
	"fmt"
	"slices"
	"strings"

	"github.com/luskaner/ageServer/common/game"
	"github.com/spf13/pflag"
)

const GameIdentifier = `game`
const GamesIdentifier = GameIdentifier + `s`
const shorthand = `e`
const descriptionStart = `Game type.`
const descriptionMultipleStart = `Game types.`
const descriptionEnd = `are supported.`

// supportedGamesDescription lists the supported games in a fixed order.
//
// The set behind it is unordered, so joining it directly made the help text
// shuffle between two runs of the same binary, which is a bad way to discover
// that a page changed.
func supportedGamesDescription() string {
	names := game.SupportedGames.ToSlice()
	slices.Sort(names)
	return strings.Join(names, ", ")
}
func GameVarCommand(flags *pflag.FlagSet, gameId *string) {
	flags.StringVarP(
		gameId,
		GameIdentifier,
		shorthand,
		"",
		fmt.Sprintf(
			`%s %s %s`,
			descriptionStart,
			supportedGamesDescription(),
			descriptionEnd,
		),
	)
}

func gamesDescription() string {
	return fmt.Sprintf(
		`%s %s %s`,
		descriptionMultipleStart,
		supportedGamesDescription(),
		descriptionEnd,
	)
}

func GamesVarCommand(flags *pflag.FlagSet, gameIds *[]string) {
	flags.StringArrayVarP(
		gameIds,
		GamesIdentifier,
		shorthand,
		game.SupportedGames.ToSlice(),
		gamesDescription(),
	)
}

func LogRootCommand(flags *pflag.FlagSet, logRoot *string) {
	flags.StringVar(
		logRoot,
		"logRoot",
		"",
		"Path to the log folder. If not empty, enables extra logging.",
	)
}

type LogRootBase interface {
	LogRootRef() *string
}

type LogRootValues struct {
	LogRoot string
}

func (v *LogRootValues) LogRootRef() *string {
	return &v.LogRoot
}

type GameIdBase interface {
	GameIdRef() *string
}

type GameIdValues struct {
	GameId string
}

func (v *GameIdValues) GameIdRef() *string {
	return &v.GameId
}
