package platform

import (
	"bufio"
	"errors"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

func IsAdmin() bool {
	f, err := os.Open("/proc/self/status")
	if err != nil {
		return os.Geteuid() == 0
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if v, ok := strings.CutPrefix(sc.Text(), "CapEff:"); ok {
			caps, err := strconv.ParseUint(strings.TrimSpace(v), 16, 64)
			return err == nil && caps&(1<<unix.CAP_NET_ADMIN) != 0
		}
	}
	return os.Geteuid() == 0
}

const lockAddr = "@OvrenNET"

var lock net.Listener

func SingleInstance() bool {
	ln, err := net.Listen("unix", lockAddr)
	if err != nil {
		return false
	}
	lock = ln
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			c.Close()
		}
	}()
	return true
}

func InstanceRunning() bool {
	c, err := net.DialTimeout("unix", lockAddr, time.Second)
	if err != nil {
		return false
	}
	c.Close()
	return true
}

func RedirectStd(f *os.File) {
	os.Stdout, os.Stderr = f, f
	log.SetOutput(f)
	unix.Dup3(int(f.Fd()), 1, 0)
	unix.Dup3(int(f.Fd()), 2, 0)
}

func RelaunchElevated(args []string, hidden bool) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	tool := "pkexec"
	if HasConsole() {
		tool = "sudo"
	}
	path, err := exec.LookPath(tool)
	if err != nil {
		return fmt.Errorf("%s not found; run OvrenNET as root or give it CAP_NET_ADMIN (see README)", tool)
	}
	return syscall.Exec(path, append([]string{tool, exe}, args...), os.Environ())
}

func RelaunchElevatedAndWait(args []string) error {
	exe, err := os.Executable()
	if err != nil {
		return err
	}
	tool := "pkexec"
	if HasConsole() {
		tool = "sudo"
	}
	path, err := exec.LookPath(tool)
	if err != nil {
		return fmt.Errorf("%s not found; run OvrenNET as root or give it CAP_NET_ADMIN (see README)", tool)
	}
	command := exec.Command(path, append([]string{exe}, args...)...)
	command.Stdin, command.Stdout, command.Stderr = os.Stdin, os.Stdout, os.Stderr
	return command.Run()
}

func RelaunchTray(args []string, hidden bool) error {
	exe, _ := os.Executable()
	if strings.HasPrefix(exe, "/nix/store/") {
		return errors.New("the tray needs CAP_NET_ADMIN: enable programs.OvrenNET in your NixOS configuration " +
			"and run /run/wrappers/bin/OvrenNET (see README). `sudo OvrenNET run` works without it")
	}
	return errors.New("the tray needs CAP_NET_ADMIN. Grant it once with:\n" +
		"  sudo setcap cap_net_admin,cap_net_raw+ep " + exe + "\n" +
		"`sudo OvrenNET run` works without it")
}

func HasConsole() bool {
	_, err := unix.IoctlGetTermios(2, unix.TCGETS)
	return err == nil
}

func ShowError(msg string) {
	if path, err := exec.LookPath("notify-send"); err == nil {
		exec.Command(path, "--urgency=critical", "--app-name=OvrenNET", "OvrenNET", msg).Run()
	}
}

func CheckDriver() error {
	if _, err := os.Stat("/dev/net/tun"); err != nil {
		return fmt.Errorf("/dev/net/tun not found (try: sudo modprobe tun): %w", err)
	}
	return nil
}

func WaitForTray(max time.Duration) bool { return true }

func OpenURL(url string) error {
	cmd := exec.Command("xdg-open", url)
	if err := cmd.Start(); err != nil {
		return err
	}
	go cmd.Wait()
	return nil
}
