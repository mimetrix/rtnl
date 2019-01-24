# rtnl

A native Go rtnetlink library based on
[this Go netlink library](https://github.com/mdlayher/netlink) by [Matt
Layher](https://mdlayher.com/).

## Why?

Why another netlink-ish library? A few reasons. This library follows the
philosophy and reasoning laid out in the
[why](https://github.com/mdlayher/netlink#why) section of the above repo and
provides an [rtnetlink](http://man7.org/linux/man-pages/man7/rtnetlink.7.html)
layer. At the time of writing we know of only
[this](https://github.com/jsimonetti/rtnetlink) repo that makes a similar
attempt for rtnetlink. But it does not appear to be actively maintined and only
covers a small part of rtnetlink. As we've been developing various Merge
components, we find ourselves often needing to interact with rtnetlink. This
library will provide factored out support for that need in Merge and anyone else
who wants to use it.

This library will stay exclusive to netlink and will not wander into iproute2 and
the like.

## Design

rtnl is structured around 4 basic rtnetlink objects.

- links
- routes
- neighbors
- addresses

There is a high level object for each of these object kinds with management
functions exposed.

Support for the remaining objects may come in time

- rules
- qdiscs
- tclass
- tfilter

## Examples

### Virtual ethernet device management

```go
// create a veth pair
ve := &Link{
  Info: &LinkInfo{
    Name: "vethA",
    Veth: &Veth{
      Peer: "vethB",
    },
  },
}
err := ve.Add()

// put one side of the pair in the pizza namespace
vb := &Link{
  Info: &LinkInfo{
    Name: "vethB",
    Ns:   netns.GetFd("pizza"),
  },
}
err = vb.Set()
```
