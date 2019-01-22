package rtnetlink

import (
	"encoding/binary"
	"fmt"
	"net"

	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Route encapsulates information about a route
type Route struct {
	Dest     net.IP
	Src      net.IP
	Gateway  net.IP
	PrefSrc  net.IP
	Oif      uint32
	Iif      uint32
	Priority uint32
	Metrics  uint32
	Family   uint8
}

// RtMsg encapsulates a unix.RtMsg providing marshal/unmarshal operations and
type RtMsg struct {
	Msg           unix.RtMsg
	RawAttributes []netlink.Attribute

	// extracted attribures
	Route
}

// Marshal a route message to bytes
func (rtm RtMsg) Marshal() ([]byte, error) {

	m := rtm.Msg

	flags := make([]byte, 4)
	binary.LittleEndian.PutUint32(flags, m.Flags)

	oif := make([]byte, 4)
	binary.LittleEndian.PutUint32(oif, rtm.Oif)

	message := []byte{
		m.Family,
		m.Dst_len,
		m.Src_len,
		m.Tos,
		m.Table,
		m.Protocol,
		m.Scope,
		m.Type,
		flags[0], flags[1], flags[2], flags[3],
	}

	attrs := []netlink.Attribute{
		// destination
		{
			Length: 4 + 4,
			Type:   unix.RTA_DST,
			Data:   rtm.Dest.To4(),
		},
		// gateway
		{
			Length: 4 + 4,
			Type:   unix.RTA_GATEWAY,
			Data:   rtm.Gateway.To4(),
		},
		// outbound interface
		{
			Length: 4 + 4,
			Type:   unix.RTA_OIF,
			Data:   oif,
		},
	}

	if rtm.Route.Metrics != 0 {

		metrics := make([]byte, 4)
		binary.LittleEndian.PutUint32(metrics, rtm.Route.Metrics)

		attrs = append(attrs, netlink.Attribute{
			Length: 4 + 4,
			Type:   unix.RTA_METRICS,
			Data:   metrics,
		})

	}
	//TODO add other known attributes if not zero values

	//attrs = append(attrs, rtm.RawAttributes...)

	attributes, err := netlink.MarshalAttributes(attrs)
	if err != nil {
		return nil, err
	}

	return append(message, attributes...), nil
}

// Unmarshal an route message and its attributes from bytes
func (rtm *RtMsg) Unmarshal(bs []byte) error {

	flags := binary.LittleEndian.Uint32(bs[8:12])

	rtm.Msg = unix.RtMsg{
		Family:   bs[0],
		Dst_len:  bs[1],
		Src_len:  bs[2],
		Tos:      bs[3],
		Table:    bs[4],
		Protocol: bs[5],
		Scope:    bs[6],
		Type:     bs[7],
		Flags:    flags,
	}

	ad, err := netlink.NewAttributeDecoder(bs[12:])
	if err != nil {
		log.WithError(err).Error("error creating decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case unix.RTA_DST:
			rtm.Dest = net.IP(ad.Bytes())
		case unix.RTA_SRC:
			rtm.Src = net.IP(ad.Bytes())
		case unix.RTA_GATEWAY:
			rtm.Gateway = net.IP(ad.Bytes())
		case unix.RTA_PREFSRC:
			rtm.PrefSrc = net.IP(ad.Bytes())
		case unix.RTA_IIF:
			rtm.Iif = ad.Uint32()
		case unix.RTA_OIF:
			rtm.Oif = ad.Uint32()
		case unix.RTA_PRIORITY:
			rtm.Priority = ad.Uint32()
		case unix.RTA_METRICS:
			rtm.Metrics = ad.Uint32()

		default:
			rtm.RawAttributes = append(rtm.RawAttributes, netlink.Attribute{
				Length: uint16(len(ad.Bytes()) + 4), // +4 for length & type fields
				Type:   ad.Type(),
				Data:   ad.Bytes(),
			})
		}
	}

	return nil

}

// read all routes from netlink returning a map that is keyed based on the
// route destination
func readRoutes() (map[string]Route, error) {

	conn, err := netlink.Dial(unix.NETLINK_ROUTE, nil)
	if err != nil {
		log.WithError(err).Error("failed to dial netlink")
		return nil, err
	}
	defer conn.Close()

	rtmsg, err := RtMsg{
		Msg: unix.RtMsg{
			Family: unix.AF_INET,
			Table:  unix.RT_TABLE_DEFAULT,
		},
	}.Marshal()
	if err != nil {
		log.WithError(err).Error("failed to marshal rtmsg")
		return nil, err
	}

	m := netlink.Message{
		Header: netlink.Header{
			Type: unix.RTM_GETROUTE,
			Flags: netlink.HeaderFlagsRequest |
				netlink.HeaderFlagsAtomic |
				netlink.HeaderFlagsRoot,
		},
		Data: rtmsg,
	}

	resp, err := conn.Execute(m)
	if err != nil {
		return nil, err
	}

	log.Debugf("current baselayer routes (%d)", len(resp))

	rts := make(map[string]Route)
	for _, r := range resp {

		var rtm RtMsg
		rtm.Unmarshal(r.Data)

		log.WithFields(log.Fields{
			"dest":    rtm.Route.Dest,
			"Gateway": rtm.Route.Gateway,
			"Oif":     rtm.Route.Oif,
		}).Debug("route")

		rtm.Route.Family = rtm.Family
		rts[rtm.Route.Dest.String()] = rtm.Route

	}

	return rts, nil

}

func addRoutes(rs []Route) error {

	return modifyRoutes(rs, unix.RTM_NEWROUTE)

}

func removeRoutes(rs []Route) error {

	return modifyRoutes(rs, unix.RTM_DELROUTE)

}

// modify a set of routes
// op=RTM_NEWROUTE ---> add
// op=RTM_DELROUTE ---> remove
func modifyRoutes(rs []Route, op uint16) error {

	// prepare netlink messages
	var messages []netlink.Message

	flags := netlink.HeaderFlagsRequest | netlink.HeaderFlagsAcknowledge
	if op == unix.RTM_NEWROUTE {
		flags |= netlink.HeaderFlagsCreate | netlink.HeaderFlagsAppend
	}

	for _, r := range rs {

		fields := log.Fields{
			"op":    op,
			"route": fmt.Sprintf("%+v", r),
		}

		switch op {

		case unix.RTM_NEWROUTE:
			log.WithFields(fields).Info("adding route")

		case unix.RTM_DELROUTE:
			log.WithFields(fields).Info("removing route")

		default:
			log.WithFields(fields).Warning("unsupported route operation, skipping")
			continue

		}

		r.Metrics = 20 //default for BGP

		msg := RtMsg{
			Msg: unix.RtMsg{
				Family:   unix.AF_INET, // r.Family, //unix.AF_INET,
				Table:    unix.RT_TABLE_MAIN,
				Type:     unix.RTN_UNICAST,
				Protocol: unix.RTPROT_BGP,
				Scope:    unix.RT_SCOPE_UNIVERSE,
				Flags:    unix.RTNH_F_ONLINK,
				Dst_len:  32,
			},
			Route: r,
		}

		data, err := msg.Marshal()
		if err != nil {
			log.WithError(err).Error("failed to marshal rtmsg")
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
	return netlinkUpdate(messages)

}
