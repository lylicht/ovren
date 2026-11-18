package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/lylicht/ovren/app/ovrennet/internal/engine"
	"github.com/lylicht/ovren/app/ovrennet/internal/logx"
	"github.com/lylicht/ovren/app/ovrennet/internal/nics"
	"github.com/lylicht/ovren/app/ovrennet/internal/platform"
	"github.com/lylicht/ovren/app/ovrennet/internal/state"
	"github.com/lylicht/ovren/app/ovrennet/internal/tun"
)

type adapterSnapshot struct {
	Name                       string  `json:"name"`
	Connected                  bool    `json:"connected"`
	DownloadBytesPerSecond     float64 `json:"downloadBytesPerSecond"`
	UploadBytesPerSecond       float64 `json:"uploadBytesPerSecond"`
	LinkDownloadBytesPerSecond float64 `json:"linkDownloadBytesPerSecond"`
	LinkUploadBytesPerSecond   float64 `json:"linkUploadBytesPerSecond"`
	ActiveFlows                int64   `json:"activeFlows"`
	TotalFlows                 int64   `json:"totalFlows"`
	State                      string  `json:"state"`
	IPv6State                  string  `json:"ipv6State"`
	Weight                     float64 `json:"weight"`
	Weight6                    float64 `json:"weight6"`
	LimitMbps                  float64 `json:"limitMbps"`
}

type networkSnapshot struct {
	Running      bool              `json:"running"`
	Enabled      bool              `json:"enabled"`
	PausedReason string            `json:"pausedReason"`
	Since        *time.Time        `json:"since,omitempty"`
	Adapters     []adapterSnapshot `json:"adapters"`
	Config       state.Config      `json:"config"`
	ConfigError  string            `json:"configError"`
	SocksRunning bool              `json:"socksRunning"`
}

func main() {
	if len(os.Args) < 2 {
		fail(errors.New("OvrenNET requires a command"))
	}
	var err error
	switch os.Args[1] {
	case "snapshot":
		err = writeSnapshot(os.Args[2:])
	case "run":
		err = run(os.Args[2:])
	case "off":
		err = turnOff(os.Args[2:])
	case "configure":
		err = configure(os.Args[2:])
	case "pause", "resume":
		err = setManualState(os.Args[1], os.Args[2:])
	case "recover":
		err = recoverRoutes(os.Args[2:])
	case "socks":
		err = runSocks(os.Args[2:])
	case "socks-off":
		err = turnOffSocks(os.Args[2:])
	default:
		err = fmt.Errorf("unknown OvrenNET command: %s", os.Args[1])
	}
	if err != nil {
		fail(err)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}

func writeSnapshot(args []string) error {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	stateDir := flags.String("state-dir", state.DefaultDir(), "")
	socksDir := flags.String("socks-state-dir", filepath.Join(state.DefaultUserDir(), "socks"), "")
	configJSON := flags.String("config-json", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	config, err := readConfigFlag(*configJSON)
	if err != nil {
		return err
	}
	out := networkSnapshot{Running: platform.InstanceRunning(), Config: config, Adapters: make([]adapterSnapshot, 0)}
	if status, ok := state.Read(*stateDir); ok && out.Running && status.Mode == "OvrenNET" {
		out.Enabled = status.Enabled
		out.PausedReason = status.PausedReason
		out.Since = &status.Since
		out.Config = status.Config
		out.ConfigError = status.ConfigError
		liveAdapters := make(map[string]engine.SlotStats, len(status.NICs))
		for _, adapter := range status.NICs {
			liveAdapters[strings.ToLower(adapter.Name)] = adapter
		}
		adapters, err := nics.DiscoverNics(nil, nil)
		if err != nil {
			return err
		}
		for _, adapter := range adapters {
			snapshot := adapterSnapshot{Name: adapter.Name, Connected: true, State: "available"}
			if live, ok := liveAdapters[strings.ToLower(adapter.Name)]; ok {
				snapshot = adapterSnapshot{
					Name:                       adapter.Name,
					Connected:                  live.Present,
					DownloadBytesPerSecond:     live.RateIn,
					UploadBytesPerSecond:       live.RateOut,
					LinkDownloadBytesPerSecond: live.LinkIn,
					LinkUploadBytesPerSecond:   live.LinkOut,
					ActiveFlows:                live.Active,
					TotalFlows:                 live.Total,
					State:                      live.State,
					IPv6State:                  live.State6,
					Weight:                     live.Weight,
					Weight6:                    live.Weight6,
					LimitMbps:                  live.LimitMbps,
				}
			}
			out.Adapters = append(out.Adapters, snapshot)
		}
	} else {
		adapters, err := nics.DiscoverNics(nil, nil)
		if err != nil {
			return err
		}
		for _, adapter := range adapters {
			out.Adapters = append(out.Adapters, adapterSnapshot{Name: adapter.Name, Connected: true})
		}
	}
	if status, ok := state.Read(*socksDir); ok && status.Mode == "socks" && platform.OwnerProcessRunning(status.PID) {
		out.SocksRunning = true
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func run(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	stateDir := flags.String("state-dir", state.DefaultDir(), "")
	ownerPID := flags.Int("owner-pid", 0, "")
	configJSON := flags.String("config-json", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *ownerPID <= 0 {
		return errors.New("OvrenNET needs a live Ovren session")
	}
	config, err := readConfigFlag(*configJSON)
	if err != nil {
		return err
	}
	if platform.InstanceRunning() {
		return nil
	}
	adapters, err := discoverAdapters(config)
	if err != nil {
		return err
	}
	if len(adapters) == 0 {
		return errors.New("No selected internet connection is available")
	}
	if !platform.IsAdmin() {
		if err := state.Prepare(*stateDir); err != nil && runtime.GOOS != "darwin" && runtime.GOOS != "linux" {
			return err
		}
		return platform.RelaunchElevated([]string{
			"run",
			"-state-dir",
			*stateDir,
			"-owner-pid",
			strconv.Itoa(*ownerPID),
			"-config-json",
			*configJSON,
		}, true)
	}
	if !platform.SingleInstance() {
		return errors.New("OvrenNET is already active")
	}
	if err := state.Prepare(*stateDir); err != nil {
		return err
	}
	if err := platform.CheckDriver(); err != nil {
		return err
	}
	state.TakeMarker(*stateDir, "off")
	state.TakeMarker(*stateDir, "quit")
	state.TakeMarker(*stateDir, "pause")
	state.TakeMarker(*stateDir, "resume")
	state.TakeConfigRequest(*stateDir)
	scheduler := engine.NewScheduler(adapters)
	scheduler.SetAuto(config.AutoWeight)
	scheduler.SetLimits(config.LimitsMbps)
	balancer := engine.NewBalancer(scheduler)
	v6Mode, _ := engine.ParseV6Mode(config.IPv6Mode)
	balancer.SetV6Mode(v6Mode)
	go scheduler.RunProbes(nil)
	device := tun.New(balancer)
	device.MTU = config.MTU
	if err := device.On(); err != nil {
		return err
	}
	logx.Printf("OvrenNET is active with %d adapters\n", len(adapters))
	since := time.Now()
	autoPause := state.AutoPause{}
	manualPaused := false
	configError := ""
	appliedConfigID := ""
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	rescan := time.NewTicker(3 * time.Second)
	defer rescan.Stop()
	last := since
	for {
		select {
		case <-stop:
			device.Close()
			state.Clear(*stateDir)
			return nil
		case <-tick.C:
			if !platform.OwnerProcessRunning(*ownerPID) || state.TakeMarker(*stateDir, "off") || state.TakeMarker(*stateDir, "quit") {
				device.Close()
				state.Clear(*stateDir)
				return nil
			}
			now := time.Now()
			if state.TakeMarker(*stateDir, "pause") {
				manualPaused = true
				device.Off()
				configError = ""
			}
			if state.TakeMarker(*stateDir, "resume") {
				manualPaused = false
				autoPause.ManualResume(now, scheduler.UsableCount())
				if err := device.On(); err != nil {
					configError = err.Error()
				} else {
					configError = ""
				}
			}
			if nextConfig, id, ok := state.TakeConfigRequest(*stateDir); ok {
				if err := applyConfig(device, scheduler, balancer, config, nextConfig); err != nil {
					configError = err.Error()
				} else {
					config = nextConfig
					appliedConfigID = id
					configError = ""
				}
			}
			scheduler.Tick(now.Sub(last))
			last = now
			pause, resume, pausedReason := autoPause.Step(now, config.AutoPause, manualPaused, scheduler.Busy(), scheduler.UsableCount())
			if pause {
				device.Off()
			}
			if resume {
				if err := device.On(); err != nil {
					configError = err.Error()
					autoPause.ResumeFailed(now)
				} else {
					configError = ""
				}
			}
			if manualPaused {
				pausedReason = "manual"
			} else if autoPause.Paused() {
				pausedReason = "automatic"
			} else if configError != "" && !device.IsOn() {
				pausedReason = "error"
			} else {
				pausedReason = "none"
			}
			state.Write(*stateDir, state.Status{
				PID:             os.Getpid(),
				Mode:            "OvrenNET",
				Enabled:         device.IsOn(),
				V6:              balancer.V6State(),
				PausedReason:    pausedReason,
				Config:          config,
				AppliedConfigID: appliedConfigID,
				ConfigError:     configError,
				Since:           since,
				Updated:         now,
				NICs:            scheduler.Snapshot(),
			})
		case <-rescan.C:
			if updated, err := discoverAdapters(config); err == nil {
				scheduler.Update(updated)
			}
		}
	}
}

func applyConfig(device *tun.Tun, scheduler *engine.Scheduler, balancer *engine.Balancer, current, next state.Config) error {
	if err := next.Validate(); err != nil {
		return err
	}
	adapters, err := discoverAdapters(next)
	if err != nil {
		return err
	}
	if len(adapters) == 0 {
		return errors.New("No selected internet connection is available")
	}
	oldMode, _ := engine.ParseV6Mode(current.IPv6Mode)
	newMode, _ := engine.ParseV6Mode(next.IPv6Mode)
	rebuild := current.MTU != next.MTU || current.IPv6Mode != next.IPv6Mode
	wasOn := device.IsOn()
	if rebuild && wasOn {
		device.Close()
	}
	scheduler.SetAuto(next.AutoWeight)
	scheduler.SetLimits(next.LimitsMbps)
	scheduler.Update(adapters)
	balancer.SetV6Mode(newMode)
	device.MTU = next.MTU
	if rebuild && wasOn {
		if err := device.On(); err != nil {
			restoreAdapters, restoreErr := discoverAdapters(current)
			if restoreErr == nil && len(restoreAdapters) > 0 {
				scheduler.SetAuto(current.AutoWeight)
				scheduler.SetLimits(current.LimitsMbps)
				scheduler.Update(restoreAdapters)
				balancer.SetV6Mode(oldMode)
				device.MTU = current.MTU
				if restartErr := device.On(); restartErr == nil {
					return fmt.Errorf("network settings were not applied: %w", err)
				} else {
					return fmt.Errorf("network settings failed and previous routes could not be restored: %v; %w", restartErr, err)
				}
			}
			balancer.SetV6Mode(oldMode)
			device.MTU = current.MTU
			return fmt.Errorf("network settings failed and no previous adapter is available: %w", err)
		}
	}
	return nil
}

func configure(args []string) error {
	flags := flag.NewFlagSet("configure", flag.ContinueOnError)
	stateDir := flags.String("state-dir", state.DefaultDir(), "")
	configJSON := flags.String("config-json", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	config, err := readConfigFlag(*configJSON)
	if err != nil {
		return err
	}
	if !platform.InstanceRunning() {
		return nil
	}
	id, err := state.WriteConfigRequest(*stateDir, config)
	if err != nil {
		return err
	}
	for range 60 {
		if status, ok := state.Read(*stateDir); ok && status.AppliedConfigID == id {
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}
	return errors.New("OvrenNET did not apply the network settings")
}

func setManualState(command string, args []string) error {
	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	stateDir := flags.String("state-dir", state.DefaultDir(), "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !platform.InstanceRunning() {
		return nil
	}
	if err := state.SendMarker(*stateDir, command); err != nil {
		return err
	}
	for range 40 {
		if status, ok := state.Read(*stateDir); ok {
			if command == "pause" && status.PausedReason == "manual" {
				return nil
			}
			if command == "resume" && status.Enabled {
				return nil
			}
		}
		time.Sleep(100 * time.Millisecond)
	}
	return fmt.Errorf("OvrenNET did not %s", command)
}

func recoverRoutes(args []string) error {
	flags := flag.NewFlagSet("recover", flag.ContinueOnError)
	stateDir := flags.String("state-dir", state.DefaultDir(), "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if platform.InstanceRunning() {
		return errors.New("Stop OvrenNET before recovering network routes")
	}
	if !platform.IsAdmin() {
		return platform.RelaunchElevatedAndWait([]string{"recover", "-state-dir", *stateDir})
	}
	if err := tun.CleanupStale(); err != nil {
		return fmt.Errorf("could not recover network routes: %w", err)
	}
	state.Clear(*stateDir)
	return nil
}

func turnOff(args []string) error {
	flags := flag.NewFlagSet("off", flag.ContinueOnError)
	stateDir := flags.String("state-dir", state.DefaultDir(), "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if !platform.InstanceRunning() {
		return nil
	}
	if err := state.SendMarker(*stateDir, "off"); err != nil {
		return err
	}
	for range 60 {
		if !platform.InstanceRunning() {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("OvrenNET did not stop")
}

func runSocks(args []string) error {
	flags := flag.NewFlagSet("socks", flag.ContinueOnError)
	stateDir := flags.String("state-dir", filepath.Join(state.DefaultUserDir(), "socks"), "")
	ownerPID := flags.Int("owner-pid", 0, "")
	configJSON := flags.String("config-json", "", "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *ownerPID <= 0 || !platform.OwnerProcessRunning(*ownerPID) {
		return errors.New("OvrenNET needs a live Ovren session")
	}
	config, err := readConfigFlag(*configJSON)
	if err != nil {
		return err
	}
	if status, ok := state.Read(*stateDir); ok && status.Mode == "socks" && platform.OwnerProcessRunning(status.PID) {
		return nil
	}
	adapters, err := discoverAdapters(config)
	if err != nil {
		return err
	}
	if len(adapters) == 0 {
		return errors.New("No selected internet connection is available")
	}
	if err := state.Prepare(*stateDir); err != nil {
		return err
	}
	listener, err := net.Listen("tcp4", "127.0.0.1:1080")
	if err != nil {
		return fmt.Errorf("SOCKS5 port 1080 on 127.0.0.1 is unavailable: %w", err)
	}
	defer listener.Close()
	scheduler := engine.NewScheduler(adapters)
	scheduler.SetAuto(config.AutoWeight)
	scheduler.SetLimits(config.LimitsMbps)
	balancer := engine.NewBalancer(scheduler)
	v6Mode, _ := engine.ParseV6Mode(config.IPv6Mode)
	balancer.SetV6Mode(v6Mode)
	go scheduler.RunProbes(nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	serveError := make(chan error, 1)
	go func() { serveError <- engine.ServeSocks5(ctx, listener, balancer) }()
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	rescan := time.NewTicker(3 * time.Second)
	defer rescan.Stop()
	since := time.Now()
	last := since
	for {
		select {
		case err := <-serveError:
			if err != nil {
				return err
			}
			state.Clear(*stateDir)
			return nil
		case <-tick.C:
			if !platform.OwnerProcessRunning(*ownerPID) || state.TakeMarker(*stateDir, "socks-off") {
				cancel()
				<-serveError
				state.Clear(*stateDir)
				return nil
			}
			now := time.Now()
			scheduler.Tick(now.Sub(last))
			last = now
			state.Write(*stateDir, state.Status{PID: os.Getpid(), Mode: "socks", Enabled: true, V6: balancer.V6State(), Config: config, Since: since, Updated: now, NICs: scheduler.Snapshot()})
		case <-rescan.C:
			if updated, err := discoverAdapters(config); err == nil {
				scheduler.Update(updated)
			}
		}
	}
}

func turnOffSocks(args []string) error {
	flags := flag.NewFlagSet("socks-off", flag.ContinueOnError)
	stateDir := flags.String("state-dir", filepath.Join(state.DefaultUserDir(), "socks"), "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	status, ok := state.Read(*stateDir)
	if !ok || status.Mode != "socks" || !platform.OwnerProcessRunning(status.PID) {
		return nil
	}
	if err := state.SendMarker(*stateDir, "socks-off"); err != nil {
		return err
	}
	for range 60 {
		if !platform.OwnerProcessRunning(status.PID) {
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return errors.New("SOCKS5 proxy did not stop")
}

func discoverAdapters(config state.Config) ([]nics.Nic, error) {
	return nics.DiscoverNics(config.IncludedAdapters, config.Weights)
}

func readConfigFlag(value string) (state.Config, error) {
	if value == "" {
		return state.DefaultConfig(), nil
	}
	return state.DecodeConfig([]byte(value))
}
