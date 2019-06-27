package rtnl

import (
	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

type Wireguard struct {
}

func (t *Wireguard) Marshal(ctx *Context) ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("wireguard"))

		return ae1.Encode()

	})

	return ae.Encode()

}

func (t *Wireguard) Unmarshal(ctx *Context, buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		log.WithError(err).Error("failed to wireguard attribute decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {
		//TODO
		}
	}

	return nil

}

func (t *Wireguard) Resolve(ctx *Context) error {

	return nil

}