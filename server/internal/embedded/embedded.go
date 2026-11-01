// Package embedded runs ageBattleServer inside the ageLAN server process and
// optionally stores the replay data it records.
package embedded

import (
	"context"
	"errors"
	"fmt"
	"net"
	"strings"
	"sync"

	"github.com/luskaner/ageBattleServer"
	"github.com/luskaner/ageServer/common/battleServer"
	"github.com/luskaner/ageServer/common/battleconfig"
	"github.com/luskaner/ageServer/common/game"
	"github.com/luskaner/ageServer/common/uuid"
	"github.com/luskaner/ageServer/server/internal/logger"
	"github.com/luskaner/ageServer/server/internal/models"
)

type Options struct {
	GameID        string
	Region        string
	Name          string
	IPv4          string
	BsPort        int
	WebSocketPort int
	OutOfBandPort int
	LogLevel      string
}

type Instance struct {
	server    *battleserver.Server
	replay    *ReplayStore
	Opts      Options
	bsPort    int
	wsPort    int
	oobPort   int
	stopOnce  sync.Once
	started   bool
	startedMu sync.Mutex
	// cancel ends the context the battle server was started with. It is the
	// server's shutdown signal and it is owned here, not by the caller: the
	// server treats its context as its lifetime, so it must not be the
	// caller's startup context, which is cancelled as soon as Start returns.
	cancel context.CancelFunc
}

func (opts *Options) Validate() error {
	if opts.Region == "" {
		return errors.New("region is required")
	}
	if _, err := uuid.Parse(opts.Region); err == nil {
		return errors.New("region must not be a UUID (it would be treated as LAN)")
	}
	if strings.TrimSpace(opts.IPv4) == "" {
		return errors.New("IPv4 is required")
	}
	if opts.IPv4 != "auto" {
		if ip := net.ParseIP(opts.IPv4); ip == nil || ip.To4() == nil {
			return fmt.Errorf("invalid IPv4 %q", opts.IPv4)
		}
	}
	if opts.BsPort < 0 || opts.BsPort > 65535 {
		return fmt.Errorf("BsPort must be 0..65535")
	}
	if opts.WebSocketPort < 0 || opts.WebSocketPort > 65535 {
		return fmt.Errorf("WebSocketPort must be 0..65535")
	}
	if opts.OutOfBandPort < 0 || opts.OutOfBandPort > 65535 {
		return fmt.Errorf("OutOfBandPort must be 0..65535")
	}
	validLevels := map[string]bool{
		"none": true, "error": true, "warning": true, "notice": true, "info": true, "debug": true,
	}
	if opts.LogLevel != "" && !validLevels[strings.ToLower(opts.LogLevel)] {
		return fmt.Errorf("invalid LogLevel %q", opts.LogLevel)
	}
	return nil
}

// Start launches the battle server and registers it in the battle server list.
// ctx is a startup context: it is checked before anything is bound, and it is
// not the server's lifetime, which lasts until Stop.
func (m *Instance) Start(ctx context.Context, certificatePairFolder string, replayOpts ReplayOptions) error {
	m.startedMu.Lock()
	defer m.startedMu.Unlock()
	if m.started {
		return errors.New("already started")
	}
	if err := ctx.Err(); err != nil {
		return fmt.Errorf("start embedded battle server: %w", err)
	}

	if err := m.Opts.Validate(); err != nil {
		return fmt.Errorf("invalid embedded battle server config: %w", err)
	}

	// Resolve TLS files using common battleconfig
	certFile, keyFile, err := battleconfig.TLSFilesFromFolder(m.Opts.GameID, certificatePairFolder)
	if err != nil {
		return err
	}

	// Build config using battleconfig.ConfigBuilder. The simulation period is
	// not configurable here: leaving it at 0 makes the builder use the game's
	// own default (common/battleconfig.SimulationPeriodDefaults).
	builder := &battleconfig.ConfigBuilder{
		GameID: m.Opts.GameID,
		Region: m.Opts.Region,
		Name:   m.Opts.Name,
		IPv4:   m.Opts.IPv4,
		Ports: battleconfig.PortConfig{
			BsPort:        m.Opts.BsPort,
			WebSocketPort: m.Opts.WebSocketPort,
			OutOfBandPort: m.Opts.OutOfBandPort,
		},
		LogLevel: m.Opts.LogLevel,
		CertFile: certFile,
		KeyFile:  keyFile,
	}

	cfg, err := builder.BuildConfig()
	if err != nil {
		return fmt.Errorf("build battle server config: %w", err)
	}

	var replayStore *ReplayStore
	if replayOpts.Enabled {
		replayStore, err = NewReplayStore(m.Opts.GameID, replayOpts, ReplayMeta{
			GameID:   m.Opts.GameID,
			Region:   m.Opts.Region,
			Name:     m.Opts.Name,
			ServerID: m.Opts.Region,
		})
		if err != nil {
			return fmt.Errorf("create replay store: %w", err)
		}
		cfg.ReplayDataHandler = func(rd battleserver.ReplayData) {
			replayStore.Append(rd.SessionID, rd.Data)
		}
	}

	// The battle server treats its context as its lifetime: when it is done,
	// the listeners close and the dispatch loop returns. The ctx received here
	// is the caller's startup context, and the server module cancels it as soon
	// as Start returns (cmd/root.go), which used to leave the instance running
	// on paper but deaf: the port was closed and no LobbyConnect was ever
	// answered. Derive a context of our own, kept until Stop.
	lifecycleCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
	m.cancel = cancel

	srv, err := battleserver.New(*cfg)
	if err != nil {
		cancel()
		m.cancel = nil
		if replayStore != nil {
			replayStore.Close()
		}
		return fmt.Errorf("create battle server: %w", err)
	}

	if err := srv.Start(lifecycleCtx); err != nil {
		cancel()
		m.cancel = nil
		if replayStore != nil {
			replayStore.Close()
		}
		return fmt.Errorf("start battle server: %w", err)
	}

	boundBS, boundWS := srv.BoundPorts()
	bsPort := int(boundBS)
	wsPort := int(boundWS)

	// Get OutOfBandPort from the built config
	oobPort := int(cfg.OutOfBandPort)

	m.server = srv
	m.replay = replayStore
	m.bsPort = bsPort
	m.wsPort = wsPort
	m.oobPort = oobPort
	m.started = true

	logger.Info("Embedded battle server started: region=%s name=%s bsPort=%d wsPort=%d oobPort=%d simPeriod=%.0fms",
		m.Opts.Region, m.Opts.Name, bsPort, wsPort, oobPort, cfg.SimulationPeriod)

	base := battleServer.Base{
		Region:        m.Opts.Region,
		Name:          m.Opts.Name,
		IPv4:          m.Opts.IPv4,
		BsPort:        bsPort,
		WebSocketPort: wsPort,
		OutOfBandPort: oobPort,
	}
	bs := &models.MainBattleServer{
		Base: base,
	}
	if m.Opts.GameID == game.AoE1 {
		bs.SetBattleServerName("omit")
		bs.SetHasOobPort(false)
	} else {
		bs.SetBattleServerName("null")
		bs.SetHasOobPort(true)
	}

	return models.RegisterEmbeddedBattleServer(m.Opts.GameID, m.Opts.Region, bs)
}

func (m *Instance) Stop() {
	m.startedMu.Lock()
	defer m.startedMu.Unlock()
	if !m.started {
		return
	}
	m.stopOnce.Do(func() {
		if m.server != nil {
			if err := m.server.Stop(); err != nil {
				logger.Warn("Stop embedded battle server %s: %s", m.Opts.Region, err)
			}
		}
		// The listeners are closed, so the context watchers they leave behind
		// can be released now. cancelling before Stop would close the port
		// behind Stop's back and lose its error reporting.
		if m.cancel != nil {
			m.cancel()
			m.cancel = nil
		}
		if m.replay != nil {
			m.replay.Close()
		}
		m.started = false
		logger.Info("Embedded battle server stopped: region=%s", m.Opts.Region)
	})
}
