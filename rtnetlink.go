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

var Version = "undefined"

type Context struct {
	f *os.File
}

func (c *Context) Fd() int {
	if c.f == nil {
		return 0
	}
	return int(c.f.Fd())
}
func (c *Context) Close() error {
	if c == nil || c.f == nil {
		return nil
	}
	return c.f.Close()
}

// OpenContext creates a context in the specified namespace
func OpenContext(namespace string) (*Context, error) {

	f, err := os.Open(fmt.Sprintf("/var/run/netns/%s", namespace))
	if err != nil {
		return nil, err
	}
	ctx := &Context{f}

	return ctx, nil

}

// OpenDefaultContext creates a context in the default namespace
func OpenDefaultContext() (*Context, error) {

	f, err := os.Open("/proc/1/ns/net")
	if err != nil {
		return nil, err
	}
	ctx := &Context{f}

	return ctx, nil
}

// Attributes is an interface that is used on all types that can be marshaled
// and unmarshaled from rtnetlink attributes
type Attributes interface {
	Marshal(*Context) ([]byte, error)
	Unmarshal(*Context, []byte) error
	Resolve(*Context) error
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

func netlinkUpdate(ctx *Context, messages []netlink.Message) error {
	return withNsNetlink(ctx.Fd(), func(c *netlink.Conn) error {

		for _, m := range messages {

			resp, err := c.Execute(m)
			if err != nil {
				return err
			}

			for _, r := range resp {

				if r.Header.Type == netlink.Error {

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
