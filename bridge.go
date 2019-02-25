package rtnl

import (
	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

const (
	IFLA_BR_UNSPEC = iota
	IFLA_BR_FORWARD_DELAY
	IFLA_BR_HELLO_TIME
	IFLA_BR_MAX_AGE
	IFLA_BR_AGEING_TIME
	IFLA_BR_STP_STATE
	IFLA_BR_PRIORITY
	IFLA_BR_VLAN_FILTERING
	IFLA_BR_VLAN_PROTOCOL
	IFLA_BR_GROUP_FWD_MASK
	IFLA_BR_ROOT_ID
	IFLA_BR_BRIDGE_ID
	IFLA_BR_ROOT_PORT
	IFLA_BR_ROOT_PATH_COST
	IFLA_BR_TOPOLOGY_CHANGE
	IFLA_BR_TOPOLOGY_CHANGE_DETECTED
	IFLA_BR_HELLO_TIMER
	IFLA_BR_TCN_TIMER
	IFLA_BR_TOPOLOGY_CHANGE_TIMER
	IFLA_BR_GC_TIMER
	IFLA_BR_GROUP_ADDR
	IFLA_BR_FDB_FLUSH
	IFLA_BR_MCAST_ROUTER
	IFLA_BR_MCAST_SNOOPING
	IFLA_BR_MCAST_QUERY_USE_IFADDR
	IFLA_BR_MCAST_QUERIER
	IFLA_BR_MCAST_HASH_ELASTICITY
	IFLA_BR_MCAST_HASH_MAX
	IFLA_BR_MCAST_LAST_MEMBER_CNT
	IFLA_BR_MCAST_STARTUP_QUERY_CNT
	IFLA_BR_MCAST_LAST_MEMBER_INTVL
	IFLA_BR_MCAST_MEMBERSHIP_INTVL
	IFLA_BR_MCAST_QUERIER_INTVL
	IFLA_BR_MCAST_QUERY_INTVL
	IFLA_BR_MCAST_QUERY_RESPONSE_INTVL
	IFLA_BR_MCAST_STARTUP_QUERY_INTVL
	IFLA_BR_NF_CALL_IPTABLES
	IFLA_BR_NF_CALL_IP6TABLES
	IFLA_BR_NF_CALL_ARPTABLES
	IFLA_BR_VLAN_DEFAULT_PVID
)

const (
	IFLA_BRIDGE_FLAGS = iota
	IFLA_BRIDGE_MODE
	IFLA_BRIDGE_VLAN_INFO
	IFLA_BRIDGE_VLAN_TUNNEL_INFO
)

const (
	BRIDGE_VLAN_INFO_MASTER = 1 << iota
	BRIDGE_VLAN_INFO_PVID
	BRIDGE_VLAN_INFO_UNTAGGED
	BRIDGE_VLAN_INFO_RANGE_BEGIN
	BRIDGE_VLAN_INFO_RANGE_END
	BRIDGE_VLAN_INFO_BRENTRY
)

type Bridge struct {
	VlanAware bool
}

func (b *Bridge) Marshal(ctx *Context) ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("bridge"))
		ae1.Do(IFLA_INFO_DATA, func() ([]byte, error) {

			ae2 := netlink.NewAttributeEncoder()
			if b.VlanAware {
				ae2.Uint8(IFLA_BR_VLAN_FILTERING, 1)
			}
			return ae2.Encode()

		})

		return ae1.Encode()

	})
	attrbuf, err := ae.Encode()
	if err != nil {
		log.WithError(err).Error("failed to encode bridge attributes")
	}

	return attrbuf, nil

}

func (b *Bridge) Unmarshal(ctx *Context, buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		log.WithError(err).Error("failed to create bridge attribute decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {

		case IFLA_BR_VLAN_FILTERING:
			value := ad.Uint8()
			if value > 0 {
				b.VlanAware = true
			}

		}
	}

	return nil

}

func (b *Bridge) Resolve(ctx *Context) error {

	return nil

}
