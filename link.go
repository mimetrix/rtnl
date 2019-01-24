package rtnl

import (
	"encoding/binary"
	"fmt"

	"github.com/mdlayher/netlink"
	"github.com/mdlayher/netlink/nlenc"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Constants ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

const (
	ifInfomsgLen = 16
)

// LinkType aliases link type enumerations in a type safe way
type LinkType uint32

const (
	UnspecLinkType LinkType = iota
	PhysicalType
	VxlanType
	VethType
)

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

// veth attribute types
const (
	VETH_INFO_UNSPEC uint16 = iota
	VETH_INFO_PEER
)

// Data Structures ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// Link consolidates link information from rtnetlink
type Link struct {
	Msg  unix.IfInfomsg
	Info *LinkInfo
}

// LinkInfo holds link attribute data
type LinkInfo struct {
	Name string

	// The following are optional link properties and are null if not present

	// network namespace file descriptor
	Ns    uint32
	Addrs []Address
	Veth  *Veth
	Vxlan *Vxlan
}

// Methods ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// Marshal turns a link into a binary rtnetlink message and a set of attributes.
func (l Link) Marshal() ([]byte, error) {

	typ := make([]byte, 2)
	binary.LittleEndian.PutUint16(typ, l.Msg.Type)

	index := make([]byte, 4)
	nlenc.PutInt32(index, l.Msg.Index)

	flags := make([]byte, 4)
	binary.LittleEndian.PutUint32(flags, l.Msg.Flags)

	change := make([]byte, 4)
	binary.LittleEndian.PutUint32(change, l.Msg.Change)

	msg := []byte{
		l.Msg.Family,
		0, //padding per include/uapi/linux/rtnetlink.h
		typ[0], typ[1],
		index[0], index[1], index[2], index[3],
		flags[0], flags[1], flags[2], flags[3],
		change[0], change[1], change[2], change[3],
	}

	ae := netlink.NewAttributeEncoder()

	if l.Info != nil && l.Info.Name != "" {
		ae.String(unix.IFLA_IFNAME, l.Info.Name)
	}
	if l.Info != nil && l.Info.Ns != 0 {
		ae.Uint32(unix.IFLA_NET_NS_FD, l.Info.Ns)
	}
	attrs, err := ae.Encode()
	if err != nil {
		return nil, err
	}

	for _, a := range l.Attributes() {

		as, err := a.Marshal()
		if err != nil {
			return nil, err
		}
		attrs = append(attrs, as...)

	}

	return append(msg, attrs...), nil

}

// Unmarshal reads a link and its attributes from a binary rtnetlink message.
func (l *Link) Unmarshal(bs []byte) error {

	typ := binary.LittleEndian.Uint16(bs[2:4])
	index := binary.LittleEndian.Uint32(bs[4:8])
	flags := binary.LittleEndian.Uint32(bs[8:12])
	change := binary.LittleEndian.Uint32(bs[12:16])

	l.Info = &LinkInfo{}

	l.Msg.Family = bs[0]
	l.Msg.Type = typ
	l.Msg.Index = int32(index)
	l.Msg.Flags = flags
	l.Msg.Change = change

	ad, err := netlink.NewAttributeDecoder(bs[16:])
	if err != nil {
		log.WithError(err).Error("error creating decoder")
		return err
	}

	var lattr Attributes
	var link uint32
	for ad.Next() {
		switch ad.Type() {

		case unix.IFLA_IFNAME:
			l.Info.Name = ad.String()

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
					attr := l.ApplyType(nad.String())
					if attr != nil {
						lattr = attr
					}

				case IFLA_INFO_DATA:
					if lattr != nil {
						lattr.Unmarshal(nad.Bytes())
					}

				}
			}

		case unix.IFLA_LINK:
			link = ad.Uint32()

		case unix.IFLA_NET_NS_FD:
			l.Info.Ns = ad.Uint32()

		}
	}

	// grap veth specific things
	veth, ok := lattr.(*Veth)
	if ok {
		veth.PeerIfx = link
	}

	// should not happen
	if l.Info.Name == "" {

		log.WithFields(log.Fields{
			"index": l.Msg.Index,
		}).Error("link has no name - this is probably a bug")

		return fmt.Errorf("no link name")

	}

	return nil

}

// ReadLinks reads a set of links according to the provided specification. For
// example, if you specify the address family, only links from that family will
// be returned. Some basic attribute filtering is also implemented.
func ReadLinks(spec *Link) ([]*Link, error) {

	var result []*Link

	m := netlink.Message{
		Header: netlink.Header{
			Type:  unix.RTM_GETLINK,
			Flags: netlink.HeaderFlagsRequest | netlink.HeaderFlagsAtomic,
		},
	}

	if spec.Msg.Index == 0 {
		m.Header.Flags |= netlink.HeaderFlagsRoot
	}

	if spec == nil {
		spec = &Link{}
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

			l := &Link{}
			err := l.Unmarshal(r.Data)
			if err != nil {
				log.WithError(err).Error("error reading link")
				return err
			}

			if l.Satisfies(spec) {
				result = append(result, l)
			}
		}

		return nil

	})

	return result, err

}

// ApplyType activates the link type defined by the provided string.
func (l *Link) ApplyType(typ string) Attributes {

	switch typ {

	case "vxlan":
		l.Info.Vxlan = &Vxlan{}
		return l.Info.Vxlan

	case "veth":
		l.Info.Veth = &Veth{}
		return l.Info.Veth

	}

	return nil

}

// Attributes returns a set of Attributes objects from the link.
func (l *Link) Attributes() []Attributes {

	var result []Attributes

	if l.Info != nil && l.Info.Veth != nil {
		result = append(result, l.Info.Veth)
	}

	if l.Info != nil && l.Info.Vxlan != nil {
		result = append(result, l.Info.Veth)
	}

	return result

}

// Modifiers ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// Add the link to the kernel.
func (l *Link) Add() error {

	return l.Modify(unix.RTM_NEWLINK)

}

// Present ensures the link is present.
func (l *Link) Present() error {

	err := l.Add()
	if err != nil && err.Error() != "file exists" {
		return err
	}
	return nil

}

// Set sets link attributes
func (l *Link) Set() error {

	return l.Modify(unix.RTM_SETLINK)

}

// Del deletes the link from the kernel.
func (l *Link) Del() error {

	return l.Modify(unix.RTM_DELLINK)

}

// Absent ensures the link is absent.
func (l *Link) Absent() error {

	err := l.Del()
	if err != nil && err.Error() != "no such device" {
		return err
	}
	return nil

}

// Modify changes the link according to the supplied operation. Supported
// operations include RTM_NEWLINK, RTM_SETLINK and RTM_DELLINK.
func (l *Link) Modify(op uint16) error {

	data, err := l.Marshal()
	if err != nil {
		log.WithError(err).Error("failed to marshal link")
		return err
	}

	// netlink wrapper

	flags := netlink.HeaderFlagsRequest |
		netlink.HeaderFlagsAcknowledge |
		netlink.HeaderFlagsExcl

	if op == unix.RTM_NEWLINK {
		flags |= netlink.HeaderFlagsCreate
	}

	m := netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(op),
			Flags: flags,
		},
		Data: data,
	}

	return netlinkUpdate([]netlink.Message{m})

}

// Satisfies returns true if this link satisfies the provided spec.
func (l *Link) Satisfies(spec *Link) bool {

	if spec == nil {
		return true
	}

	if l.Info != nil && spec.Info != nil && !stringSat(l.Info.Name, spec.Info.Name) {
		return false
	}

	if l.Info != nil && spec.Info != nil && !l.Info.Veth.Satisfies(spec.Info.Veth) {
		return false
	}

	return true

}
