package rtnl

import (
	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Vxlan encapsulates information about virtual extensible LAN devices.
type Vxlan struct {
	Vni uint32
}

// Marshal turns a vxlan into a binary rtnetlink set of attributes.
func (v *Vxlan) Marshal() ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("vxlan"))
		ae1.Do(IFLA_INFO_DATA, func() ([]byte, error) {

			ae2 := netlink.NewAttributeEncoder()
			ae2.Uint32(IFLA_VXLAN_ID, v.Vni)
			return ae2.Encode()

		})

		return ae1.Encode()

	})
	attrbuf, err := ae.Encode()
	if err != nil {
		log.WithError(err).Error("failed to encode vxlan attributes")
		return nil, err
	}

	return attrbuf, nil

}

// Unmarshal reads a vxlan from a binary set of attributes.
func (v *Vxlan) Unmarshal(buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		log.WithError(err).Error("failed to create vxlan attribute decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case IFLA_VXLAN_ID:
			v.Vni = ad.Uint32()

		}
	}

	return nil

}

// Resolve handle attributes
func (v *Vxlan) Resolve() error {

	return nil

}
