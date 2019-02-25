package rtnl

import (
	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Tap ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

type Tap struct {
}

func (t *Tap) Marshal(ctx *Context) ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("tap"))

		return ae1.Encode()

	})

	return ae.Encode()

}

func (t *Tap) Unmarshal(ctx *Context, buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		log.WithError(err).Error("failed to tap attribute decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {
		//TODO
		}
	}

	return nil

}

func (t *Tap) Resolve(ctx *Context) error {

	return nil

}

// Tun ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~

type Tun struct {
}

func (t *Tun) Marshal(ctx *Context) ([]byte, error) {

	ae := netlink.NewAttributeEncoder()
	ae.Do(unix.IFLA_LINKINFO, func() ([]byte, error) {

		ae1 := netlink.NewAttributeEncoder()
		ae1.Bytes(IFLA_INFO_KIND, []byte("tun"))

		return ae1.Encode()

	})

	return ae.Encode()

}

func (t *Tun) Unmarshal(ctx *Context, buf []byte) error {

	ad, err := netlink.NewAttributeDecoder(buf)
	if err != nil {
		log.WithError(err).Error("failed to tun attribute decoder")
		return err
	}

	for ad.Next() {
		switch ad.Type() {
		//TODO
		}
	}

	return nil

}

func (t *Tun) Resolve(ctx *Context) error {

	return nil

}
