package rtnl

import (
	"encoding/binary"
	"net"

	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Neighbor encapsulates information about neighbors
type Neighbor struct {
	Mac    net.HardwareAddr
	Vlan   uint32
	Port   uint32
	Master uint32
	If     uint32
	Ifx    string
	Dst    net.IP
	Vni    uint32
	SrcVni uint32
	Router bool
	Family uint8
}

// NdMsg is a Netlink message for communicating with the kernel about
// neighbors. The unix library does not have this one
type NdMsg struct {
	Family  uint8
	Ifindex uint32
	State   uint16
	Flags   uint8
	Type    uint8
}

// attribute types
const (
	NDA_UNSPEC uint16 = iota
	NDA_DST
	NDA_LLADDR
	NDA_CACHEINFO
	NDA_PROBES
	NDA_VLAN
	NDA_PORT
	NDA_VNI
	NDA_IFINDEX
	NDA_MASTER
	NDA_LINK_NETNSID
	NDA_SRC_VNI
)

// neighbor cache entry flags
const (
	NTF_USE         = 0x01
	NTF_SELF        = 0x02
	NTF_MASTER      = 0x04
	NTF_PROXY       = 0x08
	NTF_EXT_LEARNED = 0x10
	NTF_OFFLOADED   = 0x20
	NTF_ROUTER      = 0x80
)

// neighbor cache entry states
const (
	NUD_INCOMPLETE = 0x01
	NUD_REACHABLE  = 0x02
	NUD_STALE      = 0x04
	NUD_DELAY      = 0x08
	NUD_PROBE      = 0x10
	NUD_FAILED     = 0x20
	NUD_NOARP      = 0x40
	NUD_PERMANENT  = 0x80
	NUD_NONE       = 0x00
)

// NbrMsg encapsulates a netlink NdMsg, providing Marshal/Unmarshal support
type NbrMsg struct {
	Msg           NdMsg
	RawAttributes []netlink.Attribute

	Neighbor
}

// Marshal a neighbor message to bytes
func (n NbrMsg) Marshal() ([]byte, error) {

	ifindex := make([]byte, 4)
	binary.LittleEndian.PutUint32(ifindex, n.Msg.Ifindex)

	state := make([]byte, 2)
	binary.LittleEndian.PutUint16(state, n.Msg.State)

	message := []byte{
		n.Msg.Family,
		0, 0, 0, //padding per include/uapi/linux/neighbour.h
		ifindex[0], ifindex[1], ifindex[2], ifindex[3],
		state[0], state[1],
		n.Msg.Flags,
		n.Msg.Type,
	}

	attrs := []netlink.Attribute{}

	// mac
	if n.Neighbor.Mac != nil {
		attrs = append(attrs, netlink.Attribute{
			Length: 6 + 4,
			Type:   NDA_LLADDR,
			Data:   n.Neighbor.Mac,
		})
	}

	// dst
	if n.Neighbor.Dst != nil {
		attrs = append(attrs, netlink.Attribute{
			Length: 4 + 4,
			Type:   NDA_DST,
			Data:   n.Neighbor.Dst.To4(),
		})
	}

	//TODO add other known attributes

	attributes, err := netlink.MarshalAttributes(attrs)
	if err != nil {
		return nil, err
	}

	return append(message, attributes...), nil

}

// Unmarshal a neighbor message and its attributes from bytes
func (n *NbrMsg) Unmarshal(bs []byte) error {

	ifindex := binary.LittleEndian.Uint32(bs[4:8])
	state := binary.LittleEndian.Uint16(bs[8:10])

	n.Msg = NdMsg{
		Family:  bs[0],
		Ifindex: ifindex,
		State:   state,
		Flags:   bs[10],
		Type:    bs[11],
	}

	n.Neighbor.Router = (n.Msg.Flags&NTF_ROUTER) != 0 && (n.Msg.State&NUD_REACHABLE) != 0
	n.Neighbor.If = n.Msg.Ifindex

	ad, err := netlink.NewAttributeDecoder(bs[12:])
	if err != nil {
		log.WithError(err).Error("error creating decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case NDA_DST:
			n.Neighbor.Dst = net.IP(ad.Bytes())
		case NDA_LLADDR:
			n.Neighbor.Mac = net.HardwareAddr(ad.Bytes())
		case NDA_VLAN:
			n.Neighbor.Vlan = ad.Uint32()
		case NDA_PORT:
			n.Neighbor.Port = ad.Uint32()
		case NDA_VNI:
			n.Neighbor.Vni = ad.Uint32()
		case NDA_IFINDEX:
			n.Neighbor.If = ad.Uint32()
		case NDA_MASTER:
			n.Neighbor.Master = ad.Uint32()
		case NDA_SRC_VNI:
			n.Neighbor.SrcVni = ad.Uint32()

		case NDA_UNSPEC, NDA_CACHEINFO, NDA_PROBES, NDA_LINK_NETNSID:
			fallthrough
		default:
			n.RawAttributes = append(n.RawAttributes, netlink.Attribute{
				Length: uint16(len(ad.Bytes()) + 4), // +4 for length & type fields
				Type:   ad.Type(),
				Data:   ad.Bytes(),
			})

		}
	}

	return nil

}

// read the forwarding database (FDB), this essentially means reading all the
// neighbors in the AF_BRIDGE family
func readNeighbors(ctx *Context, family uint8) ([]Neighbor, error) {

	conn, err := netlink.Dial(unix.NETLINK_ROUTE, &netlink.Config{NetNS: ctx.Ns})
	if err != nil {
		log.WithError(err).Error("failed to dial netlink")
		return nil, err
	}
	defer conn.Close()

	// XXX working around this
	// https://lkml.org/lkml/2018/10/16/1407
	//
	// tl;dr when dumping bridge neighbors (AF_BRIDGE) we need to send netlink
	// an IfInfomsg, when dumping other types of neighbors (AF_INET[6], AF_UNSPEC)
	// we need to send netlink an NdMsg
	var data []byte
	if family == unix.AF_BRIDGE {
		data, err = Link{Msg: unix.IfInfomsg{Family: family}}.Marshal(ctx)
	} else {
		data, err = NbrMsg{Msg: NdMsg{Family: family}}.Marshal()
		if err != nil {
			return nil, err
		}
	}

	m := netlink.Message{
		Header: netlink.Header{
			Type: unix.RTM_GETNEIGH,
			Flags: netlink.HeaderFlagsRequest |
				netlink.HeaderFlagsAtomic |
				netlink.HeaderFlagsRoot,
		},
		Data: data,
	}

	resp, err := conn.Execute(m)
	if err != nil {
		return nil, err
	}

	log.Debugf("current baselayer neighbors (%d)", len(resp))

	var nbs []Neighbor
	for _, r := range resp {

		var m NbrMsg
		err := m.Unmarshal(r.Data)
		if err != nil {
			return nil, err
		}

		log.WithFields(log.Fields{
			"mac":    m.Neighbor.Mac.String(),
			"dst":    m.Neighbor.Dst.String(),
			"if":     m.Neighbor.If,
			"ifx":    m.Neighbor.Ifx,
			"master": m.Neighbor.Master,
			"router": m.Neighbor.Router,
		}).Debug("neighbor")

		m.Neighbor.Family = m.Family //family
		if m.Neighbor.Family == unix.AF_UNSPEC {
			m.Neighbor.Family = unix.AF_INET
		}
		if family == unix.AF_BRIDGE {
			m.Neighbor.Family = unix.AF_BRIDGE
		}
		nbs = append(nbs, m.Neighbor)
	}

	return nbs, nil

}

func addNeighbors(ctx *Context, ns []Neighbor) error {

	return modifyNeighbors(ctx, ns, unix.RTM_NEWNEIGH)

}

func removeNeighbors(ctx *Context, ns []Neighbor) error {

	return modifyNeighbors(ctx, ns, unix.RTM_DELNEIGH)

}

func modifyNeighbors(ctx *Context, ns []Neighbor, op uint16) error {

	// prepare netlink messages
	var messages []netlink.Message

	flags := netlink.HeaderFlagsRequest | netlink.HeaderFlagsAcknowledge
	if op == unix.RTM_NEWNEIGH {
		flags |= netlink.HeaderFlagsCreate | netlink.HeaderFlagsAppend
	}

	for _, n := range ns {

		fields := log.Fields{
			"family": n.Family,
			"op":     op,
		}

		switch op {

		case unix.RTM_NEWNEIGH:
			log.WithFields(fields).Info("adding neighbor")
			flags |= netlink.HeaderFlagsCreate | netlink.HeaderFlagsAppend

		case unix.RTM_DELNEIGH:
			log.WithFields(fields).Info("removing neighbor")

		default:
			log.WithFields(fields).Warning("unsupported neighbor operation, skipping")
		}

		msg := NbrMsg{
			Msg: NdMsg{
				Family:  n.Family,
				Ifindex: n.If,
				State:   NUD_PERMANENT,
			},
			Neighbor: n,
		}

		if n.Family == unix.AF_UNSPEC {
			msg.Msg.State |= NUD_REACHABLE
		}
		if n.Family == unix.AF_BRIDGE {
			msg.Msg.Flags |= NTF_SELF
		}

		data, err := msg.Marshal()
		if err != nil {
			log.WithError(err).Error("failed to marshal ndmsg")
			return err
		}

		m := netlink.Message{
			Header: netlink.Header{
				Type:  netlink.HeaderType(op),
				Flags: flags,
			},
			Data: data,
		}

		messages = append(messages, m)

	}

	// send netlink messages
	return netlinkUpdate(ctx, messages)

}
