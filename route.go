package rtnl

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
	Hdr      unix.RtMsg
	Dest     net.IP
	Src      net.IP
	Gateway  net.IP
	PrefSrc  net.IP
	Oif      uint32
	Iif      uint32
	Priority uint32
	Metrics  uint32
	Table    uint32
}

// Marshal a route message to bytes
func (r *Route) Marshal() ([]byte, error) {

	m := r.Hdr

	flags := make([]byte, 4)
	binary.LittleEndian.PutUint32(flags, m.Flags)

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

	ae := netlink.NewAttributeEncoder()

	if r.Dest != nil {
		ae.Bytes(unix.RTA_DST, r.Dest.To4())
	}

	if r.Src != nil {
		ae.Bytes(unix.RTA_SRC, r.Src.To4())
	}

	if r.Gateway != nil {
		ae.Bytes(unix.RTA_GATEWAY, r.Gateway.To4())
	}

	if r.PrefSrc != nil {
		ae.Bytes(unix.RTA_PREFSRC, r.PrefSrc.To4())
	}

	if r.Oif != 0 {
		ae.Uint32(unix.RTA_OIF, r.Oif)
	}

	if r.Iif != 0 {
		ae.Uint32(unix.RTA_IIF, r.Iif)
	}

	if r.Priority != 0 {
		ae.Uint32(unix.RTA_PRIORITY, r.Priority)
	}

	if r.Metrics != 0 {
		ae.Uint32(unix.RTA_METRICS, r.Metrics)
	}

	if r.Table != 0 {
		ae.Uint32(unix.RTA_TABLE, r.Table)
	}

	attributes, err := ae.Encode()
	if err != nil {
		return nil, err
	}

	return append(message, attributes...), nil
}

// Unmarshal an route message and its attributes from bytes
func (r *Route) Unmarshal(bs []byte) error {

	flags := binary.LittleEndian.Uint32(bs[8:12])

	r.Hdr = unix.RtMsg{
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
			r.Dest = net.IP(ad.Bytes())
		case unix.RTA_SRC:
			r.Src = net.IP(ad.Bytes())
		case unix.RTA_GATEWAY:
			r.Gateway = net.IP(ad.Bytes())
		case unix.RTA_PREFSRC:
			r.PrefSrc = net.IP(ad.Bytes())
		case unix.RTA_OIF:
			r.Oif = ad.Uint32()
		case unix.RTA_IIF:
			r.Iif = ad.Uint32()
		case unix.RTA_PRIORITY:
			r.Priority = ad.Uint32()
		case unix.RTA_METRICS:
			r.Metrics = ad.Uint32()
		case unix.RTA_TABLE:
			r.Table = ad.Uint32()

		}
	}

	return nil

}

func ReadRoutes(ctx *Context, spec *Route) ([]*Route, error) {

	var result []*Route

	m := netlink.Message{
		Header: netlink.Header{
			Type:  unix.RTM_GETROUTE,
			Flags: netlink.Request | netlink.Root,
		},
	}

	if spec == nil {
		spec = &Route{}
	}

	data, err := spec.Marshal()
	if err != nil {
		return nil, err
	}
	m.Data = data

	err = withNsNetlink(ctx.Fd(), func(conn *netlink.Conn) error {

		resp, err := conn.Execute(m)
		if err != nil {
			return err
		}

		for _, r := range resp {

			route := &Route{}
			err := route.Unmarshal(r.Data)
			if err != nil {
				return fmt.Errorf("error reading route: %v", err)
			}

			result = append(result, route)

		}

		return nil

	})

	if err != nil {
		return nil, err
	}

	return result, nil

}

func (r *Route) Add(ctx *Context) error {

	return r.Modify(ctx, unix.RTM_NEWROUTE)

}

func (r *Route) Present(ctx *Context) error {

	err := r.Add(ctx)
	if err != nil && err.Error() != "file exists" {
		return err
	}

	return nil

}

func (r *Route) Del(ctx *Context) error {

	return r.Modify(ctx, unix.RTM_DELROUTE)
}

func (r *Route) Absent(ctx *Context) error {

	err := r.Del(ctx)
	if err != nil && err.Error() != "no such file or directory" {
		return err
	}

	return nil

}

func (r *Route) Modify(ctx *Context, op uint16) error {

	if r.Hdr.Family == 0 {
		r.Hdr.Family = unix.AF_INET
	}
	if r.Table == 0 {
		r.Hdr.Table = unix.RT_TABLE_MAIN
	}
	if r.Hdr.Type == 0 {
		r.Hdr.Type = unix.RTN_UNICAST
	}

	data, err := r.Marshal()
	if err != nil {
		return err
	}

	flags := netlink.Request |
		netlink.Acknowledge |
		netlink.Excl

	if op == unix.RTM_NEWROUTE {
		flags |= netlink.Create | netlink.Append
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
