package rtnl

import (
	"net"

	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// vxlan attribute types
const (
	IFLA_VXLAN_UNSPEC uint16 = iota
	IFLA_VXLAN_ID
	IFLA_VXLAN_GROUP /* group or remote address */
	IFLA_VXLAN_LINK
	IFLA_VXLAN_LOCAL
	IFLA_VXLAN_TTL
	IFLA_VXLAN_TOS
	IFLA_VXLAN_LEARNING
	IFLA_VXLAN_AGEING
	IFLA_VXLAN_LIMIT
	IFLA_VXLAN_PORT_RANGE /* source port */
	IFLA_VXLAN_PROXY
	IFLA_VXLAN_RSC
	IFLA_VXLAN_L2MISS
	IFLA_VXLAN_L3MISS
	IFLA_VXLAN_PORT /* destination port */
	IFLA_VXLAN_GROUP6
	IFLA_VXLAN_LOCAL6
	IFLA_VXLAN_UDP_CSUM
	IFLA_VXLAN_UDP_ZERO_CSUM6_TX
	IFLA_VXLAN_UDP_ZERO_CSUM6_RX
	IFLA_VXLAN_REMCSUM_TX
	IFLA_VXLAN_REMCSUM_RX
	IFLA_VXLAN_GBP
	IFLA_VXLAN_REMCSUM_NOPARTIAL
	IFLA_VXLAN_COLLECT_METADATA
	IFLA_VXLAN_LABEL
	IFLA_VXLAN_GPE
	IFLA_VXLAN_TTL_INHERIT
)

// Vxlan encapsulates information about virtual extensible LAN devices.
type Vxlan struct {
	Vni      uint32
	Learning uint8
	DstPort  uint16
	Local    net.IP
	Link     uint32 // interface index
}

// Marshal turns a vxlan into a binary rtnetlink set of attributes.
func (v *Vxlan) Marshal(ctx *Context) ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("vxlan"))
		ae1.Do(IFLA_INFO_DATA, func() ([]byte, error) {

			ae2 := netlink.NewAttributeEncoder()
			ae2.Uint32(IFLA_VXLAN_ID, v.Vni)
			ae2.Uint8(IFLA_VXLAN_LEARNING, v.Learning)
			ae2.Uint16(IFLA_VXLAN_PORT, htons(v.DstPort))

			if v.Local != nil {
				local := v.Local.To4()
				if local != nil {
					ae2.Bytes(IFLA_VXLAN_LOCAL, local)
				}
			}
			//TODO ipv6 local

			if v.Link != 0 {
				ae2.Uint32(IFLA_VXLAN_LINK, v.Link)
			}

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
func (v *Vxlan) Unmarshal(ctx *Context, buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		log.WithError(err).Error("failed to create vxlan attribute decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case IFLA_VXLAN_ID:
			v.Vni = ad.Uint32()

		case IFLA_VXLAN_LEARNING:
			v.Learning = ad.Uint8()

		case IFLA_VXLAN_PORT:
			v.DstPort = ntohs(ad.Uint16())

		case IFLA_VXLAN_LOCAL:
			v.Local = net.IP(ad.Bytes())

		case IFLA_VXLAN_LINK:
			v.Link = ad.Uint32()

		}
	}

	return nil

}

// Resolve handle attributes
func (v *Vxlan) Resolve(ctx *Context) error {

	return nil

}
