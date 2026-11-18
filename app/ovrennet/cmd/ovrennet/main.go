package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"strconv"
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
	Name                   string  `json:"name"`
	Connected              bool    `json:"connected"`
	DownloadBytesPerSecond float64 `json:"downloadBytesPerSecond"`
	UploadBytesPerSecond   float64 `json:"uploadBytesPerSecond"`
	ActiveFlows            int64   `json:"activeFlows"`
	TotalFlows             int64   `json:"totalFlows"`
}

type networkSnapshot struct {
	Running  bool              `json:"running"`
	Enabled  bool              `json:"enabled"`
	Since    *time.Time        `json:"since,omitempty"`
	Adapters []adapterSnapshot `json:"adapters"`
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
	if err := flags.Parse(args); err != nil {
		return err
	}
	out := networkSnapshot{
		Running:  platform.InstanceRunning(),
		Adapters: make([]adapterSnapshot, 0),
	}
	if status, enabled := state.Read(*stateDir); enabled && out.Running {
		out.Enabled = true
		out.Since = &status.Since
		for _, adapter := range status.NICs {
			out.Adapters = append(out.Adapters, adapterSnapshot{
				Name:                   adapter.Name,
				Connected:              adapter.Present,
				DownloadBytesPerSecond: adapter.RateIn,
				UploadBytesPerSecond:   adapter.RateOut,
				ActiveFlows:            adapter.Active,
				TotalFlows:             adapter.Total,
			})
		}
	} else {
		adapters, err := nics.DiscoverNics(nil, nil)
		if err != nil {
			return err
		}
		for _, adapter := range adapters {
			out.Adapters = append(out.Adapters, adapterSnapshot{
				Name:      adapter.Name,
				Connected: true,
			})
		}
	}
	return json.NewEncoder(os.Stdout).Encode(out)
}

func run(args []string) error {
	flags := flag.NewFlagSet("run", flag.ContinueOnError)
	stateDir := flags.String("state-dir", state.DefaultDir(), "")
	ownerPID := flags.Int("owner-pid", 0, "")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *ownerPID <= 0 {
		return errors.New("OvrenNET needs a live Ovren session")
	}
	if platform.InstanceRunning() {
		return nil
	}
	adapters, err := nics.DiscoverNics(nil, nil)
	if err != nil {
		return err
	}
	if len(adapters) < 2 {
		return errors.New("OvrenNET needs at least two active internet connections")
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
	scheduler := engine.NewScheduler(adapters)
	scheduler.SetAuto(true)
	balancer := engine.NewBalancer(scheduler)
	balancer.SetV6Mode(engine.V6Auto)
	go scheduler.RunProbes(nil)
	device := tun.New(balancer)
	if err := device.On(); err != nil {
		return err
	}
	logx.Printf("OvrenNET is active with %d adapters\n", len(adapters))
	since := time.Now()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	defer signal.Stop(stop)
	tick := time.NewTicker(time.Second)
	defer tick.Stop()
	rescan := time.NewTicker(3 * time.Second)
	defer rescan.Stop()
	last := time.Now()
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
			scheduler.Tick(now.Sub(last))
			last = now
			state.Write(*stateDir, state.Status{
				PID:     os.Getpid(),
				Mode:    "OvrenNET",
				V6:      balancer.V6State(),
				Since:   since,
				Updated: now,
				NICs:    scheduler.Snapshot(),
			})
		case <-rescan.C:
			if updated, err := nics.DiscoverNics(nil, nil); err == nil {
				scheduler.Update(updated)
			}
		}
	}
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
