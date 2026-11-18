package tun

import (
	"errors"
	"fmt"
	"net"
	"net/netip"

	"github.com/vishvananda/netlink"
	"golang.org/x/sys/unix"

	"github.com/lylicht/ovren/app/ovrennet/internal/logx"
)

const openHint = "needs root or CAP_NET_ADMIN, and /dev/net/tun"

func configure(name string, _ int, ip4 netip.Prefix) (int, error) {
	link, err := netlink.LinkByName(name)
	if err != nil {
		return 0, fmt.Errorf("adapter %q did not appear: %w", name, err)
	}
	idx := link.Attrs().Index
	if err := netlink.AddrReplace(link, &netlink.Addr{IPNet: hostCIDR(ip4.String())}); err != nil {
		return idx, fmt.Errorf("set address: %w", err)
	}
	if err := netlink.LinkSetUp(link); err != nil {
		return idx, fmt.Errorf("bring up: %w", err)
	}
	for _, dst := range routes4 {
		if err := netlink.RouteReplace(tunRoute(idx, dst)); err != nil {
			return idx, fmt.Errorf("route %s: %w", dst, err)
		}
	}

	if err := netlink.AddrReplace(link, &netlink.Addr{IPNet: hostCIDR(tunIP6 + "/64"), Flags: unix.IFA_F_NODAD}); err != nil {
		logx.Printf("warning: IPv6 not captured: %v\n", err)
	} else {
		for _, dst := range routes6 {
			if err := netlink.RouteReplace(tunRoute(idx, dst)); err != nil {
				logx.Printf("warning: IPv6 route %s: %v\n", dst, err)
			}
		}
	}
	return idx, nil
}

func delRoutes(idx int) error {
	var errs []error
	for _, dst := range append(append([]string{}, routes4...), routes6...) {
		if err := netlink.RouteDel(tunRoute(idx, dst)); err != nil && !errors.Is(err, unix.ESRCH) && !errors.Is(err, unix.ENOENT) {
			errs = append(errs, fmt.Errorf("remove route %s: %w", dst, err))
		}
	}
	return errors.Join(errs...)
}

func tunRoute(idx int, dst string) *netlink.Route {
	return &netlink.Route{LinkIndex: idx, Dst: prefix(dst), Scope: netlink.SCOPE_LINK}
}

func hostCIDR(s string) *net.IPNet {
	ip, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	n.IP = ip
	return n
}

func prefix(s string) *net.IPNet {
	_, n, err := net.ParseCIDR(s)
	if err != nil {
		panic(err)
	}
	return n
}
