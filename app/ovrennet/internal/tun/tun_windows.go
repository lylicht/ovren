package tun

import (
	"fmt"
	"net"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/sys/windows"
	wun "golang.zx2c4.com/wireguard/tun"

	"github.com/lylicht/ovren/app/ovrennet/internal/logx"
	"github.com/lylicht/ovren/app/ovrennet/internal/nics"
)

const openHint = "needs admin and wintun.dll next to OvrenNET.exe"

var routes4win = [][2]string{{"0.0.0.0", "128.0.0.0"}, {"128.0.0.0", "128.0.0.0"}}

func init() {
	staleHook = restoreStrongHost
	wun.WintunTunnelType = "OvrenNET"

	if g, err := windows.GUIDFromString("{3f0d5a7e-9b1c-4e52-8a47-1c6b2d9e5f30}"); err == nil {
		wun.WintunStaticRequestedGUID = &g
	}
}

func configure(name string, mtu int, ip4 netip.Prefix) (int, error) {
	ifc, err := waitIface(name, 5*time.Second)
	if err != nil {
		return 0, err
	}
	idx := strconv.Itoa(ifc.Index)
	addr, mask := ip4.Addr().String(), net.IP(net.CIDRMask(ip4.Bits(), 32)).String()

	setAddr := func() error {
		return run("netsh", "interface", "ipv4", "set", "address", "name="+idx, "source=static",
			"address="+addr, "mask="+mask, "store=active")
	}
	err = setAddr()

	for try := 1; !hasAddr(ifc, addr); try++ {
		if try > 20 {
			if err == nil {
				err = fmt.Errorf("adapter %q did not keep its address %s", name, addr)
			}
			return ifc.Index, err
		}
		if try%4 == 0 {
			err = setAddr()
		}
		time.Sleep(250 * time.Millisecond)
	}

	startStrongHost(ifc.Index)

	mtuArg := "mtu=" + strconv.Itoa(mtu)
	if e := run("netsh", "interface", "ipv4", "set", "subinterface", "interface="+idx, mtuArg, "store=active"); e != nil {
		logx.Printf("warning: could not set IPv4 MTU: %v\n", e)
	}
	for _, dst := range routes4win {
		if err = run("route", "add", dst[0], "mask", dst[1], "0.0.0.0", "metric", "1", "if", idx); err != nil {
			return ifc.Index, err
		}
	}

	if e := run("netsh", "interface", "ipv6", "add", "address", "interface="+idx, "address="+tunIP6+"/64", "store=active"); e != nil {
		logx.Printf("warning: IPv6 not captured: %v\n", e)
	} else {
		if e := run("netsh", "interface", "ipv6", "set", "subinterface", "interface="+idx, mtuArg, "store=active"); e != nil {
			logx.Printf("warning: could not set IPv6 MTU: %v\n", e)
		}
		for _, p := range routes6 {
			if e := run("netsh", "interface", "ipv6", "add", "route", "prefix="+p, "interface="+idx, "metric=1", "store=active"); e != nil {
				logx.Printf("warning: IPv6 route %s: %v\n", p, e)
			}
		}
	}
	return ifc.Index, nil
}

func delRoutes(idx int) {
	defer stopStrongHost()
	for _, dst := range routes4win {
		run("route", "delete", dst[0], "mask", dst[1], "0.0.0.0", "if", strconv.Itoa(idx))
	}
	for _, p := range routes6 {
		run("netsh", "interface", "ipv6", "delete", "route", "prefix="+p, "interface="+strconv.Itoa(idx), "store=active")
	}
}

func run(name string, args ...string) error {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return cmdError(name, args, err, out)
	}
	return nil
}

func waitIface(name string, d time.Duration) (*net.Interface, error) {
	deadline := time.Now().Add(d)
	for {
		ifc, err := net.InterfaceByName(name)
		if err == nil {
			return ifc, nil
		}
		if time.Now().After(deadline) {
			return nil, fmt.Errorf("adapter %q did not appear: %w", name, err)
		}
		time.Sleep(50 * time.Millisecond)
	}
}

func hasAddr(ifc *net.Interface, ip string) bool {
	addrs, _ := ifc.Addrs()
	for _, a := range addrs {
		if n, ok := a.(*net.IPNet); ok && n.IP.String() == ip {
			return true
		}
	}
	return false
}

var strongHost struct {
	mu      sync.Mutex
	checked map[int]bool
	restore []string
	stop    chan struct{}
}

var strongHostFile = filepath.Join(os.TempDir(), "OvrenNET-weakhost.txt")

func startStrongHost(tunIdx int) {
	stop := make(chan struct{})
	strongHost.mu.Lock()
	strongHost.checked, strongHost.restore, strongHost.stop = map[int]bool{tunIdx: true}, nil, stop
	strongHost.mu.Unlock()
	syncStrongHost(stop)
	go func() {
		t := time.NewTicker(5 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-stop:
				return
			case <-t.C:
				syncStrongHost(stop)
			}
		}
	}()
}

func syncStrongHost(stop chan struct{}) {
	found, _ := nics.DiscoverNics(nil, nil)
	strongHost.mu.Lock()
	defer strongHost.mu.Unlock()
	if strongHost.stop != stop {
		return
	}
	var idx []string
	for _, n := range found {
		if !strongHost.checked[n.Index] {
			strongHost.checked[n.Index] = true
			idx = append(idx, strconv.Itoa(n.Index))
		}
	}
	if len(idx) == 0 {
		return
	}

	ps := exec.Command("powershell", "-NoProfile", "-NonInteractive", "-Command",
		"Get-NetIPInterface -InterfaceIndex "+strings.Join(idx, ",")+" -ErrorAction SilentlyContinue | "+
			"Where-Object { $_.WeakHostSend -eq 'Enabled' -or $_.Forwarding -eq 'Enabled' } | "+
			"ForEach-Object { '{0} {1} {2} {3}' -f $_.AddressFamily, $_.InterfaceIndex, $_.WeakHostSend, $_.Forwarding }")
	ps.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	out, err := ps.Output()
	if err != nil {
		logx.Printf("warning: could not check the adapters' weak host setting: %v\n", err)
		return
	}
	for _, line := range strings.Split(strings.TrimSpace(string(out)), "\n") {
		f := strings.Fields(line)
		if len(f) != 4 {
			continue
		}
		fam := strings.ToLower(f[0])
		if fam != "ipv4" && fam != "ipv6" {
			continue
		}
		if err := run("netsh", "interface", fam, "set", "interface", "interface="+f[1],
			"weakhostsend=disabled", "forwarding=disabled", "store=active"); err != nil {
			logx.Printf("warning: %v\n", err)
			continue
		}
		strongHost.restore = append(strongHost.restore, fmt.Sprintf("%s %s weakhostsend=%s forwarding=%s",
			fam, f[1], strings.ToLower(f[2]), strings.ToLower(f[3])))
		logx.Printf("note: turned off weak host send and forwarding on adapter %s (%s) while merging; "+
			"Windows ignores pinned connections there. Sharing its connection (Mobile Hotspot) pauses until OvrenNET is off.\n", f[1], f[0])
	}
	os.WriteFile(strongHostFile, []byte(strings.Join(strongHost.restore, "\n")), 0o600)
}

func stopStrongHost() {
	strongHost.mu.Lock()
	defer strongHost.mu.Unlock()
	if strongHost.stop != nil {
		close(strongHost.stop)
		strongHost.stop = nil
	}
	putBack(strongHost.restore)
	strongHost.restore = nil
	os.Remove(strongHostFile)
}

func restoreStrongHost() {
	b, err := os.ReadFile(strongHostFile)
	if err != nil {
		return
	}
	putBack(strings.Split(strings.TrimSpace(string(b)), "\n"))
	os.Remove(strongHostFile)
}

func putBack(lines []string) {
	for _, l := range lines {
		f := strings.Fields(l)
		if len(f) == 4 {
			run("netsh", "interface", f[0], "set", "interface", "interface="+f[1], f[2], f[3], "store=active")
		}
	}
}
