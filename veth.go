package rtnl

import (
	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Veth encapsulates information about virtual ethernet devices
type Veth struct {
	Peer    string
	PeerIfx uint32
}

// Marshal turns a veth into a binary rtnetlink set of attributes.
func (v *Veth) Marshal() ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("veth"))
		ae1.Do(IFLA_INFO_DATA, func() ([]byte, error) {

			ae2 := netlink.NewAttributeEncoder()
			ae2.Do(VETH_INFO_PEER, func() ([]byte, error) {

				ae3 := netlink.NewAttributeEncoder()
				ae3.String(unix.IFLA_IFNAME, v.Peer)
				buf, err := ae3.Encode()

				// VETH_INFO_PEER seems to requre sufficient leading padding to hold an
				// ifinfomsg, probably to contain ifinfo about the peer interface, this
				// information was gleaned off looking at other netlink code that deals
				// with veths such as iproute2 and lxc, I cannot find any documentation
				// that says this is how it is.
				pad := make([]byte, ifInfomsgLen)
				buf = append(pad, buf...)
				return buf, err

			})

			return ae2.Encode()

		})

		return ae1.Encode()

	})
	attrbuf, err := ae.Encode()
	if err != nil {
		log.WithError(err).Error("failed to encode veth attributes")
		return nil, err
	}

	return attrbuf, nil

}

// Unmarshal reads a veth from a binary set of attributes.
func (v *Veth) Unmarshal(buf []byte) error {

	log.Println("vern fonk?")
	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		return err
	}
	log.Println("honk?")

	for ad.Next() {
		log.Println("kerdlonk")
		switch ad.Type() {

		case VETH_INFO_PEER:

			ad1, err := netlink.NewAttributeDecoder(ad.Bytes())
			if err != nil {
				return err
			}

			log.Println("schlonk")
			for ad1.Next() {
				switch ad1.Type() {

				case unix.IFLA_IFNAME:
					log.Println("bonk")
					v.Peer = ad1.String()

				}
			}

		}
	}

	return nil

}

// Satisfies returns true if this veth satisfies the provided spec
func (v *Veth) Satisfies(spec *Veth) bool {

	if spec == nil {
		return true
	}

	if v == nil {
		return false
	}

	if !stringSat(v.Peer, spec.Peer) {
		return false
	}

	return true

}

// ResolvePeer fills in this veth's peer interface name from its index.
func (v *Veth) ResolvePeer() {

	spec := &Link{}
	spec.Msg.Index = int32(v.PeerIfx)
	result, err := ReadLinks(spec)
	if err != nil {
		log.WithError(err).Error("read links failed")
	}

	if len(result) == 0 {
		log.WithFields(log.Fields{"index": v.PeerIfx}).Error("peer does not exist")
		return
	}
	if len(result) > 1 {
		log.WithFields(log.Fields{"index": v.PeerIfx}).Error("multiple peers")
	}

	v.Peer = result[0].Info.Name

}
