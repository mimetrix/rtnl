package rtnetlink

import (
	"encoding/binary"
	"fmt"

	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

/*---------------------------------------------------------------------------*\
 * rtnetlink support
 * .................
 *
 * This file contains a set of functions and types to interact with Linux
 * netlink. There are three basic categories of things.
 *  - routes
 *  - neighbors
 *  - links
 *
 * Each category contains
 *  - functions for reading the state of objects within the category
 *  - functions for setting the state of objects within the category
 *  - data structures to facilitate netlink i/o + marshal/unmarshal funcs
\*---------------------------------------------------------------------------*/

func withNetlink(f func(*netlink.Conn) error) error {

	conn, err := netlink.Dial(unix.NETLINK_ROUTE, nil)
	if err != nil {
		log.WithError(err).Error("failed to dial netlink")
		return err
	}
	defer conn.Close()

	return f(conn)

}

func netlinkUpdate(messages []netlink.Message) error {
	return withNetlink(func(c *netlink.Conn) error {

		for _, m := range messages {

			resp, err := c.Execute(m)
			if err != nil {
				log.WithError(err).Error("netlink call failed")
				return err
			}

			for _, r := range resp {

				if r.Header.Type == netlink.HeaderTypeError {

					code := binary.LittleEndian.Uint32(r.Data[0:4])

					if code == 0 {
						log.Debug("netlink update acknowledged")
					} else {
						log.WithFields(log.Fields{
							"code": code,
						}).Warn("netlink update failed")
						return fmt.Errorf(string(r.Data))
					}

				}
			}

		}

		return nil

	})
}
