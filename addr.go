package rtnetlink

import (
	"encoding/binary"
	"net"

	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Data Structures ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// Address consolidates address information from rtnetlink.
type Address struct {
	Msg  unix.IfAddrmsg
	Info *AddrInfo
}

// AddrInfo holds address attribute data.
type AddrInfo struct {
	Address   net.IP
	Local     net.IP
	Label     string
	Broadcast net.IP
	Anycast   net.IP
}

// Marshal turns an address into a binary rtnetlink message and a set of
// attributes.
func (a Address) Marshal() ([]byte, error) {

	index := make([]byte, 4)
	binary.LittleEndian.PutUint32(index, a.Msg.Index)

	buf := []byte{
		a.Msg.Family,
		a.Msg.Prefixlen,
		a.Msg.Flags,
		a.Msg.Scope,
		index[0], index[1], index[2], index[3],
	}

	ae := netlink.NewAttributeEncoder()

	if a.Info.Address != nil {
		ae.Bytes(unix.IFA_ADDRESS, a.Info.Address)
	}
	if a.Info.Local != nil {
		ae.Bytes(unix.IFA_LOCAL, a.Info.Local)
	}
	if a.Info.Label != "" {
		ae.String(unix.IFA_LABEL, a.Info.Label)
	}
	if a.Info.Broadcast != nil {
		ae.Bytes(unix.IFA_BROADCAST, a.Info.Broadcast)
	}
	if a.Info.Anycast != nil {
		ae.Bytes(unix.IFA_ANYCAST, a.Info.Anycast)
	}

	attrs, err := ae.Encode()
	if err != nil {
		log.WithError(err).Error("failed to encode address attributes")
		return nil, err
	}

	return append(buf, attrs...), nil

}

// Unmarshal reads an address and its attributes from a binary rtnetlink
// message.
func (a *Address) Unmarshal(buf []byte) error {

	index := binary.LittleEndian.Uint32(buf[4:8])

	a.Msg.Family = buf[0]
	a.Msg.Prefixlen = buf[1]
	a.Msg.Flags = buf[2]
	a.Msg.Scope = buf[3]
	a.Msg.Index = index

	ad, err := netlink.NewAttributeDecoder(buf[8:])
	if err != nil {
		log.WithError(err).Error("error creating address decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case unix.IFA_ADDRESS:
			a.Info.Address = net.IP(ad.Bytes())

		case unix.IFA_LOCAL:
			a.Info.Local = net.IP(ad.Bytes())

		case unix.IFA_LABEL:
			a.Info.Label = ad.String()

		case unix.IFA_BROADCAST:
			a.Info.Broadcast = net.IP(ad.Bytes())

		case unix.IFA_ANYCAST:
			a.Info.Anycast = net.IP(ad.Bytes())

		}
	}

	return nil

}

// ReadAddrs reads a set of addresses according to the provided specification.
// For example, if you specify the address family, only addresses from that
// family will be returned. Some basic attribute filtering is also implemented.
func ReadAddrs(spec *Address) ([]*Address, error) {

	var result []*Address

	m := netlink.Message{
		Header: netlink.Header{
			Type: unix.RTM_GETADDR,
			Flags: netlink.HeaderFlagsRequest |
				netlink.HeaderFlagsAtomic |
				netlink.HeaderFlagsRoot,
		},
	}

	if spec == nil {
		spec = &Address{}
	}
	data, err := spec.Marshal()
	if err != nil {
		log.WithError(err).Error("failed to marshal spec link")
		return nil, err
	}
	m.Data = data

	err = withNetlink(func(conn *netlink.Conn) error {

		resp, err := conn.Execute(m)
		if err != nil {
			return err
		}

		for _, r := range resp {

			a := &Address{}
			err := a.Unmarshal(r.Data)
			if err != nil {
				log.WithError(err).Error("error reading address")
				return err
			}

			result = append(result, a)

		}

		return nil

	})

	return result, err

}
