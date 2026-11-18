package tun

import (
	"context"
	"fmt"
	"net"
	"net/netip"
	"strings"
	"sync"
	"time"

	"gvisor.dev/gvisor/pkg/tcpip/stack"

	"github.com/xjasonlyu/tun2socks/v2/core"
	"github.com/xjasonlyu/tun2socks/v2/core/device"
	"github.com/xjasonlyu/tun2socks/v2/core/device/tun"
	"github.com/xjasonlyu/tun2socks/v2/core/option"
	tlog "github.com/xjasonlyu/tun2socks/v2/log"
	M "github.com/xjasonlyu/tun2socks/v2/metadata"
	"github.com/xjasonlyu/tun2socks/v2/proxy/reject"
	"github.com/xjasonlyu/tun2socks/v2/tunnel"

	"github.com/lylicht/ovren/app/ovrennet/internal/engine"
	"github.com/lylicht/ovren/app/ovrennet/internal/logx"
)

const (
	tunName = "OvrenNET"

	tunIP6 = "fd6e:6d65:7267::1"
)

var tunRange = netip.MustParsePrefix("198.18.0.0/15")

func pickNet(used []netip.Prefix) netip.Prefix {
	var spare netip.Prefix
	for i := range 1 << (24 - tunRange.Bits()) {
		b := tunRange.Addr().As4()
		b[1] += byte(i >> 8)
		b[2], b[3] = byte(i), 1
		p := netip.PrefixFrom(netip.AddrFrom4(b), 24)
		free, held := true, false
		for _, u := range used {
			free = free && !p.Overlaps(u)
			held = held || u.Addr() == p.Addr()
		}
		if free {
			return p
		}
		if !held && !spare.IsValid() {
			spare = p
		}
	}
	if spare.IsValid() {
		return spare
	}
	return netip.PrefixFrom(tunRange.Addr().Next(), 24)
}

func usedNets(skip string) []netip.Prefix {
	ifs, _ := net.Interfaces()
	var out []netip.Prefix
	for _, ifc := range ifs {
		if ifc.Name == skip {
			continue
		}
		addrs, _ := ifc.Addrs()
		for _, a := range addrs {
			n, ok := a.(*net.IPNet)
			if !ok || n.IP.To4() == nil {
				continue
			}
			ones, bits := n.Mask.Size()
			if bits == 128 {
				ones -= 96
			}
			out = append(out, netip.PrefixFrom(netip.AddrFrom4([4]byte(n.IP.To4())), ones))
		}
	}
	return out
}

func cmdError(name string, args []string, err error, out []byte) error {
	cmdline := name + " " + strings.Join(args, " ")
	if msg := strings.Join(strings.Fields(string(out)), " "); msg != "" {
		return fmt.Errorf("%s (%s: %v)", msg, cmdline, err)
	}
	return fmt.Errorf("%s: %v", cmdline, err)
}

var openName = tunName

var staleHook func()

var (
	routes4 = []string{"0.0.0.0/1", "128.0.0.0/1"}
	routes6 = []string{"::/1", "8000::/1"}
)

type tunProxy struct{ b *engine.Balancer }

func (p tunProxy) DialContext(ctx context.Context, m *M.Metadata) (net.Conn, error) {
	return p.b.DialPinned(ctx, m.DstIP.Unmap().String(), int(m.DstPort))
}

func (p tunProxy) DialUDP(m *M.Metadata) (net.PacketConn, error) {
	f := engine.Fam4
	if d := m.DstIP.Unmap(); d.Is6() {
		f = engine.Fam6
	}
	return p.b.ListenUDP(context.Background(), f)
}

type Tun struct {
	mu  sync.Mutex
	B   *engine.Balancer
	MTU int
	dev device.Device
	stk *stack.Stack
	idx int
	on  bool
}

const DefaultMTU = 9000

const udpSessionTimeout = 30 * time.Second

func New(b *engine.Balancer) *Tun { return &Tun{B: b, MTU: DefaultMTU} }

func (t *Tun) IsOn() bool {
	t.mu.Lock()
	defer t.mu.Unlock()
	return t.on
}

var keepDevice bool

func (t *Tun) open() error {

	if lvl, e := tlog.ParseLevel("error"); e == nil {
		tlog.SetLogger(tlog.Must(tlog.NewLeveled(lvl)))
	}
	tunnel.T().SetUDPTimeout(udpSessionTimeout)
	CleanupStale()

	dev, err := tun.Open(openName, uint32(t.MTU))
	if err != nil {
		return fmt.Errorf("open TUN (%s): %w", openHint, err)
	}
	stk, err := core.CreateStack(&core.Config{
		LinkEndpoint:     dev,
		TransportHandler: tunnel.T(),
		Options:          []option.Option{option.WithTCPModerateReceiveBuffer(true)},
	})
	if err != nil {
		dev.Close()
		return err
	}
	installForwarders(stk, t.B.V6Allowed)
	t.dev, t.stk = dev, stk
	return nil
}

func (t *Tun) Warm() error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if !keepDevice || t.dev != nil {
		return nil
	}
	return t.open()
}

func (t *Tun) On() (err error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.on {
		return nil
	}
	if t.dev == nil {
		if err = t.open(); err != nil {
			return err
		}
	}
	defer func() {
		if err != nil {
			t.teardown()
		}
	}()
	tunnel.T().SetProxy(tunProxy{t.B})

	ip4 := pickNet(usedNets(t.dev.Name()))
	if ip4.Addr() != tunRange.Addr().Next() {
		logx.Printf("note: another adapter is on %s, using %s\n", netip.PrefixFrom(tunRange.Addr(), 24), ip4)
	}
	if t.idx, err = configure(t.dev.Name(), t.MTU, ip4); err != nil {
		return err
	}
	t.on = true
	return nil
}

func (t *Tun) Off() {
	t.mu.Lock()
	defer t.mu.Unlock()
	if keepDevice && t.dev != nil {
		if t.idx != 0 {
			delRoutes(t.idx)
		}
		tunnel.T().SetProxy(&reject.Reject{})
		t.on = false
		return
	}
	t.teardown()
}

func (t *Tun) Close() {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.teardown()
}

func (t *Tun) teardown() {
	if t.idx != 0 {
		delRoutes(t.idx)
	}
	tunnel.T().SetProxy(&reject.Reject{})
	if t.dev != nil {
		t.dev.Close()
	}
	if t.stk != nil {
		done := make(chan struct{})
		go func() { t.stk.Close(); t.stk.Wait(); close(done) }()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}
	t.dev, t.stk, t.idx, t.on = nil, nil, 0, false
}

func CleanupStale() {
	if staleHook != nil {
		staleHook()
	}
	if ifc, err := net.InterfaceByName(tunName); err == nil {
		delRoutes(ifc.Index)
	}
}
