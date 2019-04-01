package rtnl

import (
	"fmt"

	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

const (
	IFLA_MACVLAN_UNSPEC uint16 = iota
	IFLA_MACVLAN_MODE
	IFLA_MACVLAN_FLAGS
	IFLA_MACVLAN_MACADDR_MODE
	IFLA_MACVLAN_MACADDR
	IFLA_MACVLAN_MACADDR_DATA
	IFLA_MACVLAN_MACADDR_COUNT
)

type MacvlanMode uint32

const (
	MACVLAN_MODE_PRIVATE MacvlanMode = 1 << iota
	MACVLAN_MODE_VEPA
	MACVLAN_MODE_BRIDGE
	MACVLAN_MODE_PASSTHRU
	MACVLAN_MODE_SOURCE
)

const (
	MACVLAN_MACADDR_ADD uint32 = iota
	MACVLAN_MACADDR_DEL
	MACVLAN_MACADDR_FLUSH
	MACVLAN_MACADDR_SET
)

type Macvlan struct {
	Mode MacvlanMode
	Link uint32
}

func (m *Macvlan) Marshal(ctx *Context) ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Uint32(unix.IFLA_LINK, m.Link)
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("macvlan"))
		ae1.Do(IFLA_INFO_DATA, func() ([]byte, error) {

			ae2 := netlink.NewAttributeEncoder()
			ae2.Uint32(IFLA_MACVLAN_MODE, uint32(m.Mode))

			return ae2.Encode()

		})

		return ae1.Encode()

	})

	return ae.Encode()

}

func (m *Macvlan) Unmarshal(ctx *Context, buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case IFLA_MACVLAN_MODE:
			m.Mode = MacvlanMode(ad.Uint32())

		}
	}

	return nil

}

func (m *Macvlan) Resolve(ctx *Context) error {

	return nil

}

func ParseMacvlanMode(mode string) (MacvlanMode, error) {

	switch mode {
	case "private":
		return MACVLAN_MODE_PRIVATE, nil
	case "vepa":
		return MACVLAN_MODE_VEPA, nil
	case "bridge":
		return MACVLAN_MODE_BRIDGE, nil
	case "passthru":
		return MACVLAN_MODE_PASSTHRU, nil
	case "source":
		return MACVLAN_MODE_SOURCE, nil
	}

	return 0, fmt.Errorf("undefined macvlan mode")

}
