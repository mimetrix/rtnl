package rtnetlink

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Link info
type Link struct {
	Name    string
	Index   uint32
	Vni     uint32
	IPAddrs []net.IP
}

// interface link address attribute types
const (
	IFLA_INFO_UNSPEC uint16 = iota
	IFLA_INFO_KIND
	IFLA_INFO_DATA
)

// vxlan attribute types
const (
	IFLA_VXLAN_UNSPEC uint16 = iota
	IFLA_VXLAN_ID
)

// TODO(ry) wrap this up in a package-local structure and provide same
// Marshal/Unmarshal interface as the rest of the netlink messages
//
// marshal an interface address message to bytes
func marshalIfaddrMsg(m unix.IfAddrmsg) []byte {

	index := make([]byte, 4)
	binary.LittleEndian.PutUint32(index, m.Index)

	return []byte{
		m.Family,
		m.Prefixlen,
		m.Flags,
		m.Scope,
		index[0], index[1], index[2], index[3],
	}

}

// TODO(ry) wrap this up in a package-local structure and provide same
// Marshal/Unmarshal interface as the rest of the netlink messages
//
// unmarshal an interface address message and its attributes
func unmarshalIfaddrMsg(bs []byte) (unix.IfAddrmsg, []byte) {

	index := binary.LittleEndian.Uint32(bs[4:8])

	msg := unix.IfAddrmsg{
		Family:    bs[0],
		Prefixlen: bs[1],
		Flags:     bs[2],
		Scope:     bs[3],
		Index:     index,
	}

	return msg, bs[8:]
}

// read links information from netlink, returning a map of link objects keyed on
// the interface number. it's not the case that links are monotonically
// increasing in index, so the map is required.
func readLinks() (map[uint32]*Link, error) {

	conn, err := netlink.Dial(unix.NETLINK_ROUTE, nil)
	if err != nil {
		log.WithError(err).Error("failed to dial network")
		return nil, err
	}
	defer conn.Close()

	m := netlink.Message{
		Header: netlink.Header{
			Type: unix.RTM_GETLINK,
			Flags: netlink.HeaderFlagsRequest |
				netlink.HeaderFlagsAtomic |
				netlink.HeaderFlagsRoot,
		},
		Data: marshalIfinfoMsg(unix.IfInfomsg{
			Family: unix.AF_INET,
		}),
	}

	resp, err := conn.Execute(m)
	if err != nil {
		return nil, err
	}
	log.Debugf("current baselayer links (%d)", len(resp))

	lks := make(map[uint32]*Link)
	for _, r := range resp {

		m, attrs := unmarshalIfinfoMsg(r.Data)
		if err != nil {
			log.WithError(err).Error("error reading ifinfo")
			return nil, err
		}

		lnk := &Link{
			Index: uint32(m.Index),
		}
		err := lnk.Unmarshal(attrs)
		if err != nil {
			continue
		}

		lks[lnk.Index] = lnk
	}

	// now that we have the links, go get their addresses
	err = readLinkAddrs(lks)
	if err != nil {
		return nil, err
	}

	for _, l := range lks {
		log.WithFields(log.Fields{
			"index":   l.Index,
			"name":    l.Name,
			"vni":     l.Vni,
			"ipaddrs": fmt.Sprintf("%+v", l.IPAddrs),
		}).Debug("link info")
	}

	return lks, nil

}

// Unmarshal a link from attributes
func (lnk *Link) Unmarshal(attrs []byte) error {

	ad, err := netlink.NewAttributeDecoder(attrs)
	if err != nil {
		log.WithError(err).Error("error creating decoder")
		return err
	}

	// keep track of the current attribute kind, to get down to the vxlan
	// attributes we have to spelunk through a few layers of attributes, most of
	// the nested attribute types we don't care about and don't need to recurse
	// down into. because of the sequential read nature of interacting with
	// netlink attribute sets, we need to keep track of what attribute we are
	// currently at and look back later when we hit an IFLA_INFO_DATA attribute
	// to see if it's a data attribute we need to recurse into
	var currentKind string
	for ad.Next() {
		switch ad.Type() {

		case unix.IFLA_IFNAME:
			lnk.Name = ad.String()

		case unix.IFLA_LINKINFO:

			// always dive into linkinfo
			nad, err := netlink.NewAttributeDecoder(ad.Bytes())
			if err != nil {
				log.WithError(err).Warning("failed to create nested decoder")
				continue
			}
			for nad.Next() {
				switch nad.Type() {

				// keep track of the current attribute kind
				case IFLA_INFO_KIND:
					currentKind = nad.String()

				case IFLA_INFO_DATA:
					// only interested in vxlan things at the moment, if we hit a data
					// attribute and were in a vxlan context, dive in
					if currentKind != "vxlan" {
						continue
					}
					nnad, err := netlink.NewAttributeDecoder(nad.Bytes())
					if err != nil {
						log.WithError(err).Warning("failed to create 2x nested decoder")
						continue
					}

					// iterate through the vxlan info attributes
					for nnad.Next() {
						switch nnad.Type() {

						case IFLA_VXLAN_ID:
							// w00t, found the VNI
							lnk.Vni = nnad.Uint32()

						}
					}

				}
			}

		}
	}

	// should not happen
	if lnk.Name == "" {

		log.WithFields(log.Fields{
			"index": lnk.Index,
		}).Warn("link has no name - this is probably a bug")

		return fmt.Errorf("no link name")

	}

	return nil

}

// ask netlink for all addresses it knows about and update the provided set of
// links with addresses sets matched over link (interface) index.
func readLinkAddrs(links map[uint32]*Link) error {

	conn, err := netlink.Dial(unix.NETLINK_ROUTE, nil)
	if err != nil {
		log.WithError(err).Error("failed to dial network")
		return err
	}
	defer conn.Close()

	m := netlink.Message{
		Header: netlink.Header{
			Type: unix.RTM_GETADDR,
			Flags: netlink.HeaderFlagsRequest |
				netlink.HeaderFlagsAtomic |
				netlink.HeaderFlagsRoot,
		},
		Data: marshalIfaddrMsg(unix.IfAddrmsg{
			Family: unix.AF_INET,
		}),
	}

	resp, err := conn.Execute(m)
	if err != nil {
		return err
	}

	log.Debugf("link addresses (%d)", len(resp))

	for _, r := range resp {

		m, attrs := unmarshalIfaddrMsg(r.Data)
		if err != nil {
			log.WithError(err).Error("error reading ifaddr")
			return err
		}

		ad, err := netlink.NewAttributeDecoder(attrs)
		if err != nil {
			log.WithError(err).Error("error creating decoder")
			return err
		}

		for ad.Next() {
			switch ad.Type() {

			case unix.IFA_ADDRESS:

				// try to get a link from the provided map that matches the interface
				// index returned by netlink, if found add the address to that links
				// address list
				link, ok := links[m.Index]
				if !ok {
					log.WithFields(log.Fields{
						"index": m.Index,
					}).Warning("unknown interface index")
					continue
				}
				link.IPAddrs = append(link.IPAddrs, net.IP(ad.Bytes()))

			}
		}

	}

	return nil

}
