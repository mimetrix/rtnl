package rtnl

import (
	"github.com/mdlayher/netlink"
	"golang.org/x/sys/unix"
)

const (
	IFLA_VRF_UNSPEC uint16 = iota
	IFLA_VRF_TABLE
)

const (
	IFLA_VRF_PORT_UNSPEC uint16 = iota
	IFLA_VRF_PORT_TABLE
)

type Vrf struct {
	Table uint32
}

func (v *Vrf) Marshal(ctx *Context) ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("vrf"))
		ae1.Do(IFLA_INFO_DATA, func() ([]byte, error) {

			ae2 := netlink.NewAttributeEncoder()
			ae2.Uint32(IFLA_VRF_TABLE, v.Table)

			return ae2.Encode()

		})

		return ae1.Encode()

	})

	return ae.Encode()

}

func (v *Vrf) Unmarshal(ctx *Context, buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case IFLA_VRF_TABLE:
			v.Table = ad.Uint32()

		}
	}

	return nil

}

func (v *Vrf) Resolve(ctx *Context) error {

	return nil

}
