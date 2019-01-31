package rtnl

import (
	"encoding/binary"
	"fmt"
	"os"
	"runtime"

	"github.com/mdlayher/netlink"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sys/unix"
)

// Attributes is an interface that is used on all types that can be marshaled
// and unmarshaled from rtnetlink attributes
type Attributes interface {
	Marshal() ([]byte, error)
	Unmarshal([]byte) error
	Resolve() error
}

func withNetlink(f func(*netlink.Conn) error) error {

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	thisNS, err := os.Open(
		fmt.Sprintf("/proc/%d/task/%d/ns/net", os.Getpid(), unix.Gettid()))
	if err != nil {
		log.WithError(err).Error("failed to open this netns")
		return fmt.Errorf("failed to open netns")
	}
	defer thisNS.Close()

	conn, err := netlink.Dial(
		unix.NETLINK_ROUTE, &netlink.Config{NetNS: int(thisNS.Fd())})
	if err != nil {
		log.WithError(err).Error("failed to dial netlink")
		return err
	}
	defer conn.Close()

	return f(conn)

}

func withNsNetlink(ns int, f func(*netlink.Conn) error) error {

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	conn, err := netlink.Dial(
		unix.NETLINK_ROUTE, &netlink.Config{NetNS: ns})
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
				return err
			}

			for _, r := range resp {

				if r.Header.Type == netlink.HeaderTypeError {

					code := binary.LittleEndian.Uint32(r.Data[0:4])

					// code == 0 is just an acknowledgement
					if code != 0 {
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
