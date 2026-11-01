//go:build !windows

package watch

import "context"

func rebroadcastBattleServer(_ context.Context, _ *ExitCode, _ int) {}
