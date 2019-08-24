package rtnl

import (
	"encoding/binary"
	"fmt"
	"net"
	"strings"

	"golang.org/x/sys/unix"

	"github.com/mdlayher/netlink"
)

const (
	FRA_UNSPEC uint16 = iota
	FRA_DST
	FRA_SRC
	FRA_IIFNAME
	FRA_GOTO
	FRA_UNUSED2
	FRA_PRIORITY
	FRA_UNUSED3
	FRA_UNUSED4
	FRA_UNUSED5
	FRA_FWMARK
	FRA_FLOW
	FRA_TUN_ID
	FRA_SUPPRESS_IFGROUP
	FRA_SUPPRESS_PREFIXLEN
	FRA_TABLE
	FRA_FWMASK
	FRA_OIFNAME
	FRA_PAD
	FRA_L3MDEV
	FRA_UID_RANGE
	FRA_PROTOCOL
	FRA_IP_PROTO
	FRA_SPORT_RANGE
	FRA_DPORT_RANGE
)

const (
	FR_ACT_UNSPEC uint8 = iota
	FR_ACT_TO_TBL
	FR_ACT_GOTO
	FR_ACT_NOP
	FR_ACT_RES3
	FR_ACT_RES4
	FR_ACT_BLACKHOLE
	FR_ACT_UNREACHABLE
	FR_ACT_PROHIBIT
)

type Rule struct {
	Fib      Fib
	Priority uint32
	Src      net.IP
	Dest     net.IP
	Oif      string
	Iif      string
	Fwmark   uint32
	Table    uint32
}

type Fib struct {
	Family uint8
	DstLen uint8
	SrcLen uint8
	Tos    uint8
	Table  uint8
	Res1   uint8
	Res2   uint8
	Action uint8
	Flags  uint32
}

func (r *Rule) Marshal(ctx *Context) ([]byte, error) {

	// defaults
	if r.Fib.Action == 0 {
		r.Fib.Action = FR_ACT_TO_TBL
	}

	// header

	flags := make([]byte, 4)
	binary.LittleEndian.PutUint32(flags, r.Fib.Flags)

	if r.Table < 256 {
		r.Fib.Table = uint8(r.Table)
	}

	hdr := []byte{
		r.Fib.Family,
		r.Fib.DstLen,
		r.Fib.SrcLen,
		r.Fib.Tos,
		r.Fib.Table,
		r.Fib.Res1,
		r.Fib.Res2,
		r.Fib.Action,
		flags[0], flags[1], flags[2], flags[3],
	}

	// attributes

	ae := netlink.NewAttributeEncoder()

	if r.Priority != 0 {
		ae.Uint32(FRA_PRIORITY, r.Priority)
	}

	if r.Src != nil {
		ae.Bytes(FRA_SRC, r.Src.To4())
	}

	if r.Dest != nil {
		ae.Bytes(FRA_DST, r.Dest.To4())
	}

	if r.Oif != "" {
		ae.String(FRA_OIFNAME, r.Oif)
	}

	if r.Iif != "" {
		ae.String(FRA_IIFNAME, r.Iif)
	}

	if r.Fwmark != 0 {
		ae.Uint32(FRA_FWMARK, r.Fwmark)
	}

	if r.Table >= 256 {
		ae.Uint32(FRA_TABLE, r.Table)
	}

	attrs, err := ae.Encode()
	if err != nil {
		return nil, err
	}

	return append(hdr, attrs...), nil

}

func (r *Rule) Unmarshal(ctx *Context, buf []byte) error {

	// header

	r.Fib.Family = buf[0]
	r.Fib.DstLen = buf[1]
	r.Fib.SrcLen = buf[2]
	r.Fib.Tos = buf[3]
	r.Fib.Table = buf[4]
	r.Fib.Res1 = buf[5]
	r.Fib.Res2 = buf[6]
	r.Fib.Action = buf[7]
	r.Fib.Flags = binary.LittleEndian.Uint32(buf[8:12])

	// attributes

	ad, err := netlink.NewAttributeDecoder(buf[12:])
	if err != nil {
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case FRA_PRIORITY:
			r.Priority = ad.Uint32()

		case FRA_OIFNAME:
			r.Oif = ad.String()

		case FRA_IIFNAME:
			r.Iif = ad.String()

		case FRA_TABLE:
			r.Table = ad.Uint32()

		case FRA_SRC:
			r.Src = ad.Bytes()

		case FRA_DST:
			r.Dest = ad.Bytes()

		case FRA_FWMARK:
			r.Fwmark = ad.Uint32()

		}
	}

	return nil

}

func (r *Rule) Resolve(ctx *Context) error { return nil }

func ReadRules(ctx *Context, spec *Rule) ([]*Rule, error) {

	var result []*Rule

	m := netlink.Message{
		Header: netlink.Header{
			Type:  unix.RTM_GETRULE,
			Flags: netlink.Request | netlink.Root,
		},
	}

	if spec == nil {
		spec = &Rule{}
	}

	data, err := spec.Marshal(ctx)
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

			rule := &Rule{}
			err := rule.Unmarshal(ctx, r.Data)
			if err != nil {
				return fmt.Errorf("error reading rule: %v", err)
			}

			result = append(result, rule)

		}

		return nil

	})

	if err != nil {
		return nil, err
	}

	return result, nil

}

func (r *Rule) Add(ctx *Context) error {

	return r.Modify(ctx, unix.RTM_NEWRULE)

}

func (r *Rule) Present(ctx *Context) error {

	err := r.Add(ctx)

	if err != nil && !strings.Contains(err.Error(), "file exists") {
		return err
	}

	return nil

}

func (r *Rule) Del(ctx *Context) error {

	return r.Modify(ctx, unix.RTM_DELRULE)

}

func (r *Rule) Absent(ctx *Context) error {

	err := r.Del(ctx)

	if err != nil && strings.Contains(err.Error(), "no such file") {
		return err
	}

	return nil

}

func (r *Rule) Modify(ctx *Context, op uint16) error {

	data, err := r.Marshal(ctx)
	if err != nil {
		return err
	}

	flags := netlink.Request |
		netlink.Acknowledge |
		netlink.Excl

	if op == unix.RTM_NEWRULE {
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
