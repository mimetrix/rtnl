package rtnl

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"

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
	LoopbackType
	PhysicalType
	VxlanType
	VethType
	BridgeType
	TapType
	TunType
	VrfType
	MacvlanType
	WireguardType
)

// interface link address attribute types
const (
	IFLA_INFO_UNSPEC uint16 = iota
	IFLA_INFO_KIND
	IFLA_INFO_DATA
)

// Data Structures ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// Link consolidates link information from rtnetlink
type Link struct {
	Msg  unix.IfInfomsg
	Info *LinkInfo
}

// NewLink creates a new empty link data structure
func NewLink() *Link {
	return &Link{Info: &LinkInfo{}}
}

// LinkInfo holds link attribute data
type LinkInfo struct {
	// Name of the link
	Name string

	// layer 2 address
	Address net.HardwareAddr

	Promisc bool

	// network namespace file descriptor
	Ns uint32

	// maximum transmission unit
	Mtu uint32

	// the network namespace the link is in
	LinkNS uint32

	// bridge master
	Master uint32

	// vlan-aware bridge properties
	Pvid     uint16
	Untagged []uint16
	Tagged   []uint16

	// loopback properties
	Loopback *Loopback

	// veth properties
	Veth *Veth

	// vxlan properties
	Vxlan *Vxlan

	// bridge properties
	Bridge *Bridge

	// tap properties
	Tap *Tap

	// tun properties
	Tun *Tun

	// vrf properties
	Vrf *Vrf

	// macvlan properties
	Macvlan *Macvlan

	// wireguard properties
	Wireguard *Wireguard
}

// Methods ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// Marshal turns a link into a binary rtnetlink message and a set of attributes.
func (l Link) Marshal(ctx *Context) ([]byte, error) {

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

	if l.Msg.Change == 0 {
		ae := netlink.NewAttributeEncoder()

		if l.Info != nil {
			if l.Info.Name != "" {
				ae.String(unix.IFLA_IFNAME, l.Info.Name)
			}
			if l.Info.Master != 0 {
				ae.Uint32(unix.IFLA_MASTER, l.Info.Master)
			}
			if l.Info.Ns != 0 {
				ae.Uint32(unix.IFLA_NET_NS_FD, l.Info.Ns)
			}
			if l.Info.Address != nil && !isZeroMac(l.Info.Address) {
				ae.Bytes(unix.IFLA_ADDRESS, l.Info.Address)
			}
			if l.Info.Mtu != 0 {
				ae.Uint32(unix.IFLA_MTU, l.Info.Mtu)
			}
			if l.Msg.Family == unix.AF_BRIDGE {
				ae.Uint32(unix.IFLA_EXT_MASK, 2)
			}
		}
		attrs, err := ae.Encode()
		if err != nil {
			return nil, err
		}

		for _, a := range l.Attributes() {

			as, err := a.Marshal(ctx)
			if err != nil {
				return nil, err
			}
			attrs = append(attrs, as...)

		}

		return append(msg, attrs...), nil
	}

	return msg, nil

}

// Unmarshal reads a link and its attributes from a binary rtnetlink message.
func (l *Link) Unmarshal(ctx *Context, bs []byte) error {

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

	if (l.Msg.Flags & unix.IFF_PROMISC) != 0 {
		l.Info.Promisc = true
	}

	if (l.Msg.Flags & unix.IFF_LOOPBACK) != 0 {
		l.Info.Loopback = &Loopback{}
	}

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

		case unix.IFLA_MASTER:
			l.Info.Master = ad.Uint32()

		case unix.IFLA_ADDRESS:
			l.Info.Address = net.HardwareAddr(ad.Bytes())

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
						lattr.Unmarshal(ctx, nad.Bytes())
					}

				}
			}

		case unix.IFLA_AF_SPEC:

			nad, err := netlink.NewAttributeDecoder(ad.Bytes())
			if err != nil {
				log.WithError(err).Warning("failed to create bridge spec decoder")
				continue
			}
			for nad.Next() {
				switch nad.Type() {

				case IFLA_BRIDGE_VLAN_INFO:
					bs := nad.Bytes()
					flags := nlenc.Uint16(bs[:2])
					vid := nlenc.Uint16(bs[2:4])

					if (flags & BRIDGE_VLAN_INFO_PVID) != 0 {
						l.Info.Pvid = vid
					}

					if (flags & BRIDGE_VLAN_INFO_UNTAGGED) != 0 {
						l.Info.Untagged = append(l.Info.Untagged, vid)
					} else {
						l.Info.Tagged = append(l.Info.Tagged, vid)
					}
				}
			}

		case unix.IFLA_LINK:
			link = ad.Uint32()

		case unix.IFLA_MTU:
			l.Info.Mtu = ad.Uint32()

		case unix.IFLA_NET_NS_FD:
			l.Info.Ns = ad.Uint32()

		case unix.IFLA_LINK_NETNSID:
			l.Info.LinkNS = ad.Uint32()

		}
	}

	// grap veth specific things
	veth, ok := lattr.(*Veth)
	if ok {
		veth.PeerIfx = link
	}

	// grap macvlan specific things
	macvlan, ok := lattr.(*Macvlan)
	if ok {
		macvlan.Link = link
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

func (l *Link) SetMtu(ctx *Context, mtu int) error {

	if mtu == 0 {
		return nil
	}

	err := l.Read(ctx)
	if err != nil {
		return err
	}

	msg := IfInfomsgBytes(l.Msg)

	ae := netlink.NewAttributeEncoder()
	ae.Uint32(unix.IFLA_MTU, uint32(mtu))

	attrs, err := ae.Encode()
	if err != nil {
		return err
	}

	data := append(msg, attrs...)

	flags := netlink.Request |
		netlink.Acknowledge |
		netlink.Excl

	m := netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(unix.RTM_SETLINK),
			Flags: flags,
		},
		Data: data,
	}

	l.Info.Mtu = uint32(mtu)

	return netlinkUpdate(ctx, []netlink.Message{m})

}

func (l *Link) SetMaster(ctx *Context, index int) error {

	if index == 0 {
		return nil
	}

	err := l.Read(ctx)
	if err != nil {
		return err
	}
	l.Msg.Change |= unix.IFF_MASTER
	l.Msg.Flags |= unix.IFF_MASTER

	msg := IfInfomsgBytes(l.Msg)

	ae := netlink.NewAttributeEncoder()
	ae.Uint32(unix.IFLA_MASTER, uint32(index))

	attrs, err := ae.Encode()
	if err != nil {
		return err
	}

	data := append(msg, attrs...)

	flags := netlink.Request |
		netlink.Acknowledge |
		netlink.Excl

	m := netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(unix.RTM_SETLINK),
			Flags: flags,
		},
		Data: data,
	}

	return netlinkUpdate(ctx, []netlink.Message{m})

}

// ReadLinks reads a set of links according to the provided specification. For
// example, if you specify the address family, only links from that family will
// be returned. Some basic attribute filtering is also implemented.
func ReadLinks(ctx *Context, spec *Link) ([]*Link, error) {

	var result []*Link

	m := netlink.Message{
		Header: netlink.Header{
			Type:  unix.RTM_GETLINK,
			Flags: netlink.Request,
		},
	}

	if spec == nil {
		spec = &Link{}
	}

	if spec.Msg.Family == unix.AF_BRIDGE {
		m.Header.Flags |= netlink.Dump
	} else {
		m.Header.Flags |= netlink.Atomic
	}

	if spec.Msg.Index == 0 {
		m.Header.Flags |= netlink.Root
	}

	data, err := spec.Marshal(ctx)
	if err != nil {
		log.WithError(err).Error("failed to marshal spec link")
		return nil, err
	}
	m.Data = data

	err = withNsNetlink(ctx.Fd(), func(conn *netlink.Conn) error {

		resp, err := conn.Execute(m)
		if err != nil {
			return err
		}

		for _, r := range resp {

			l := &Link{}
			err := l.Unmarshal(ctx, r.Data)
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

func (l *Link) Read(ctx *Context) error {

	spec := NewLink()
	spec.Msg.Family = l.Msg.Family
	spec.Msg.Index = l.Msg.Index

	if l.Info != nil {
		spec.Info.Name = l.Info.Name
	}

	links, err := ReadLinks(ctx, spec)
	if err != nil {
		return err
	}

	if len(links) == 0 {
		return fmt.Errorf("not found")
	}
	if len(links) > 1 {
		return fmt.Errorf("not unique")
	}

	if l.Msg.Family == unix.AF_BRIDGE {
		l.Info.Untagged = links[0].Info.Untagged
		l.Info.Tagged = links[0].Info.Tagged
		l.Info.Pvid = links[0].Info.Pvid
	} else {
		*l = *links[0]
	}

	for _, a := range l.Attributes() {
		err := a.Resolve(ctx)
		if err != nil {
			return err
		}
	}

	return nil

}

func GetLink(ctx *Context, name string) (*Link, error) {
	link := &Link{
		Info: &LinkInfo{
			Name: name,
		},
	}
	err := link.Read(ctx)

	return link, err
}

func GetLinkByIndex(ctx *Context, index int32) (*Link, error) {
	link := &Link{
		Msg: unix.IfInfomsg{
			Index: index,
		},
	}
	err := link.Read(ctx)

	return link, err
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

	case "bridge":
		l.Info.Bridge = &Bridge{}
		return l.Info.Bridge

	case "tap":
		l.Info.Tap = &Tap{}
		return l.Info.Tap

	case "tun":
		l.Info.Tun = &Tun{}
		return l.Info.Tun

	case "vrf":
		l.Info.Vrf = &Vrf{}
		return l.Info.Vrf

	case "macvlan":
		l.Info.Macvlan = &Macvlan{}
		return l.Info.Macvlan

	case "wireguard":
		l.Info.Wireguard = &Wireguard{}
		return l.Info.Wireguard

	}

	log.Tracef("unknown type %s", typ)

	return nil

}

func (li *LinkInfo) Type() LinkType {

	if li.Loopback != nil {
		return LoopbackType
	}
	if li.Veth != nil {
		return VethType
	}
	if li.Vxlan != nil {
		return VxlanType
	}
	if li.Bridge != nil {
		return BridgeType
	}
	if li.Tap != nil {
		return TapType
	}
	if li.Tun != nil {
		return TunType
	}
	if li.Vrf != nil {
		return VrfType
	}
	if li.Macvlan != nil {
		return MacvlanType
	}
	if li.Wireguard != nil {
		return WireguardType
	}

	//TODO Is this a reasonable default? Given the logic of how types are
	//ascertained i think its at least decent.
	return PhysicalType

}

// Attributes returns a set of Attributes objects from the link.
func (l *Link) Attributes() []Attributes {

	var result []Attributes

	if l.Info != nil && l.Info.Veth != nil {
		result = append(result, l.Info.Veth)
	}

	if l.Info != nil && l.Info.Vxlan != nil {
		result = append(result, l.Info.Vxlan)
	}

	if l.Info != nil && l.Info.Bridge != nil {
		result = append(result, l.Info.Bridge)
	}

	if l.Info != nil && l.Info.Vrf != nil {
		result = append(result, l.Info.Vrf)
	}

	if l.Info != nil && l.Info.Macvlan != nil {
		result = append(result, l.Info.Macvlan)
	}

	if l.Info != nil && l.Info.Wireguard != nil {
		result = append(result, l.Info.Wireguard)
	}

	return result

}

// Modifiers ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

// Add the link to the kernel.
func (l *Link) Add(ctx *Context) error {

	err := l.Modify(ctx, unix.RTM_NEWLINK)
	if err != nil {
		return err
	}

	// read kernel info about the link
	if ctx.Target != nil {
		return l.Read(ctx.Target)
	} else {
		return l.Read(ctx)
	}

}

// Present ensures the link is present.
func (l *Link) Present(ctx *Context) error {

	err := l.Add(ctx)

	if err != nil {
		if !strings.Contains(err.Error(), "file exists") {
			return err
		}
		if ctx.Target != nil {
			return l.Read(ctx.Target)
		} else {
			return l.Read(ctx)
		}
	}

	return nil

}

// Set sets link attributes
func (l *Link) Set(ctx *Context) error {

	return l.Modify(ctx, unix.RTM_SETLINK)

}

// Del deletes the link from the kernel.
func (l *Link) Del(ctx *Context) error {

	return l.Modify(ctx, unix.RTM_DELLINK)

}

// Absent ensures the link is absent.
func (l *Link) Absent(ctx *Context) error {

	err := l.Del(ctx)
	if err != nil && !strings.Contains(err.Error(), "no such device") {
		return err
	}
	return nil

}

// Up brings up the link
func (l *Link) Up(ctx *Context) error {

	err := l.Read(ctx)
	if err != nil {
		return nil
	}

	if l.Msg.Flags&unix.IFF_UP == 0 {
		l.Msg.Change |= unix.IFF_UP
		l.Msg.Flags |= unix.IFF_UP
		return l.Modify(ctx, unix.RTM_SETLINK)
	}

	return nil

}

// Up down brings down the link
func (l *Link) Down(ctx *Context) error {

	err := l.Read(ctx)
	if err != nil {
		return nil
	}

	if l.Msg.Flags&unix.IFF_UP != 0 {
		l.Msg.Change |= unix.IFF_UP
		l.Msg.Flags &= ^uint32(unix.IFF_UP)
		return l.Modify(ctx, unix.RTM_SETLINK)
	}

	return nil

}

func (l *Link) Promisc(ctx *Context, v bool) error {

	// promisc
	if v {
		l.Msg.Change |= unix.IFF_PROMISC
		l.Msg.Flags |= unix.IFF_PROMISC
	} else {
		l.Msg.Change |= unix.IFF_PROMISC
		l.Msg.Flags &= ^uint32(unix.IFF_PROMISC)
	}

	return l.Modify(ctx, unix.RTM_SETLINK)

}

// Modify changes the link according to the supplied operation. Supported
// operations include RTM_NEWLINK, RTM_SETLINK and RTM_DELLINK.
func (l *Link) Modify(ctx *Context, op uint16) error {

	data, err := l.Marshal(ctx)
	if err != nil {
		log.WithError(err).Error("failed to marshal link")
		return err
	}

	// netlink wrapper

	flags := netlink.Request |
		netlink.Acknowledge |
		netlink.Excl

	if op == unix.RTM_NEWLINK {
		flags |= netlink.Create
	}

	m := netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(op),
			Flags: flags,
		},
		Data: data,
	}

	return netlinkUpdate(ctx, []netlink.Message{m})

}

func (l *Link) SetUntagged(ctx *Context, vid uint16, unset bool, pvid bool, self bool) error {

	return l.SetVlan(ctx, vid, unset, true, pvid, self)

}

func (l *Link) SetTagged(ctx *Context, vid uint16, unset bool, pvid bool, self bool) error {

	return l.SetVlan(ctx, vid, unset, false, pvid, self)

}

func (l *Link) SetVlan(ctx *Context, vid uint16, unset, untagged, pvid, self bool) error {

	if vid == 0 {
		return nil
	}

	orig := l.Msg.Family
	l.Msg.Family = unix.AF_BRIDGE
	defer func() { l.Msg.Family = orig }()

	err := l.Read(ctx)
	if err != nil {
		return err
	}

	msg := IfInfomsgBytes(l.Msg)

	ae := netlink.NewAttributeEncoder()

	if l.Info == nil {
		return fmt.Errorf("no link info")
	}

	if !unset {
		if untagged {
			for _, x := range l.Info.Untagged {
				if x == vid {
					//already set
					return nil
				}
			}
			l.Info.Untagged = append(l.Info.Untagged, vid)
		} else {
			for _, x := range l.Info.Tagged {
				if x == vid {
					//already set
					return nil
				}
			}
			l.Info.Tagged = append(l.Info.Tagged, vid)
		}
	} else {

		found := false
		if untagged {
			for _, x := range l.Info.Untagged {
				if x == vid {
					found = true
					break
				}
			}
		} else {
			for _, x := range l.Info.Tagged {
				if x == vid {
					found = true
					break
				}
			}
		}

		if !found {
			// nothing to do
			return nil
		}
	}

	ae.Do(unix.IFLA_AF_SPEC, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Do(IFLA_BRIDGE_VLAN_INFO, func() ([]byte, error) {

			var fl uint16 = 0
			if untagged {
				fl |= BRIDGE_VLAN_INFO_UNTAGGED
			}
			if pvid {
				fl |= BRIDGE_VLAN_INFO_PVID
			}

			flags := nlenc.Uint16Bytes(fl)
			evid := nlenc.Uint16Bytes(vid)
			return append(flags, evid...), nil

		})

		if self {
			ae1.Uint16(IFLA_BRIDGE_FLAGS, BRIDGE_FLAGS_SELF)
		}
		return ae1.Encode()

	})

	attrs, err := ae.Encode()
	if err != nil {
		return err
	}

	data := append(msg, attrs...)

	flags := netlink.Request |
		netlink.Acknowledge |
		netlink.Excl

	op := unix.RTM_SETLINK
	if unset {
		op = unix.RTM_DELLINK
	}

	m := netlink.Message{
		Header: netlink.Header{
			Type:  netlink.HeaderType(op),
			Flags: flags,
		},
		Data: data,
	}

	return netlinkUpdate(ctx, []netlink.Message{m})

}

func (l *Link) AddAddr(ctx *Context, addr *Address) error {

	addr.Msg.Index = uint32(l.Msg.Index)
	return AddAddr(ctx, addr)

}

// Satisfies returns true if this link satisfies the provided spec.
func (l *Link) Satisfies(spec *Link) bool {

	if spec == nil {
		return true
	}

	if l.Info != nil &&
		spec.Info != nil &&
		!stringSat(l.Info.Name, spec.Info.Name) {
		return false
	}

	if l.Info != nil &&
		spec.Info != nil &&
		!l.Info.Veth.Satisfies(spec.Info.Veth) {
		return false
	}

	return true

}

func (lt LinkType) String() string {

	switch lt {
	case PhysicalType:
		return "physical"
	case LoopbackType:
		return "loopback"
	case VxlanType:
		return "vxlan"
	case VethType:
		return "veth"
	case BridgeType:
		return "bridge"
	case TapType:
		return "tap"
	case TunType:
		return "tun"
	case VrfType:
		return "vrf"
	case MacvlanType:
		return "macvlan"
	case WireguardType:
		return "wireguard"
	default:
		return "unspec"
	}

}

func ParseLinkType(str string) LinkType {

	switch str {
	case "physical":
		return PhysicalType
	case "loopback":
		return LoopbackType
	case "vxlan":
		return VxlanType
	case "veth":
		return VethType
	case "bridge":
		return BridgeType
	case "tap":
		return TapType
	case "tun":
		return TunType
	case "vrf":
		return VrfType
	case "macvlan":
		return MacvlanType
	case "wireguard":
		return WireguardType
	default:
		return UnspecLinkType
	}

}

func IfInfomsgBytes(msg unix.IfInfomsg) []byte {

	typ := make([]byte, 2)
	binary.LittleEndian.PutUint16(typ, msg.Type)

	index := make([]byte, 4)
	nlenc.PutInt32(index, msg.Index)

	flags := make([]byte, 4)
	binary.LittleEndian.PutUint32(flags, msg.Flags)

	change := make([]byte, 4)
	binary.LittleEndian.PutUint32(change, msg.Change)

	return []byte{
		msg.Family,
		0, //padding per include/uapi/linux/rtnetlink.h
		typ[0], typ[1],
		index[0], index[1], index[2], index[3],
		flags[0], flags[1], flags[2], flags[3],
		change[0], change[1], change[2], change[3],
	}

}

func (l *Link) Addrs(ctx *Context) ([]*Address, error) {

	return ReadAddrs(ctx, &Address{
		Msg: unix.IfAddrmsg{
			Index: uint32(l.Msg.Index),
		},
	})

}
