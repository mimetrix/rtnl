package rtnl

import (
	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Loopback ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

type Loopback struct {
}

func (t *Loopback) Marshal(ctx *Context) ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("loopback"))

		return ae1.Encode()

	})

	return ae.Encode()

}

func (t *Loopback) Unmarshal(ctx *Context, buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		log.WithError(err).Error("failed to loopback attribute decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {
		//TODO
		}
	}

	return nil

}

func (t *Loopback) Resolve(ctx *Context) error {

	return nil

}
