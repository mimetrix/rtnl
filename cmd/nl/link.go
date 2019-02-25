package main

import (
	"fmt"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"gitlab.com/mergetb/tech/rtnl"
)

func linkCommands(root *cobra.Command) {

	link := &cobra.Command{
		Use:   "link",
		Short: "link command family",
	}
	root.AddCommand(link)

	// list
	var (
		typ    string
		bridge string
	)
	list := &cobra.Command{
		Use:   "list",
		Short: "list links",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { doList(typ, bridge) },
	}
	list.Flags().StringVarP(&typ, "type", "t", "", "filter on link type")
	list.Flags().StringVarP(&bridge, "bridge", "b", "", "filter on bridge")
	link.AddCommand(list)

	// delete
	delete := &cobra.Command{
		Use:   "delete <name>",
		Short: "delete a link",
		Args:  cobra.ExactArgs(1),
		Run:   func(cmd *cobra.Command, args []string) { doDelete(args[0]) },
	}
	link.AddCommand(delete)

	// add
	add := &cobra.Command{
		Use:   "add",
		Short: "add a link",
	}
	link.AddCommand(add)

	// addBridge
	var (
		brinfo *rtnl.Bridge = &rtnl.Bridge{}
	)
	addBridge := &cobra.Command{
		Use:   "bridge <name>",
		Short: "add a bridge",
		Args:  cobra.ExactArgs(1),
		Run:   func(cmd *cobra.Command, args []string) { doAddBridge(args[0], brinfo) },
	}
	addBridge.Flags().BoolVarP(
		&brinfo.VlanAware, "vlan-aware", "v", false, "vlan aware bridge")
	add.AddCommand(addBridge)

	// addVxlan
	var (
		vxinfo   *rtnl.Vxlan = &rtnl.Vxlan{}
		learning bool
		local    string
		vxlink   string
		vxbr     string
		untagged uint16
	)
	addVxlan := &cobra.Command{
		Use:   "vxlan <name> <vni>",
		Short: "add vxlan",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			if learning {
				vxinfo.Learning = 1
			}
			if local != "" {
				vxinfo.Local = net.ParseIP(local)
			}
			vni, err := strconv.Atoi(args[1])
			if err != nil {
				log.Fatal(err)
			}
			vxinfo.Vni = uint32(vni)
			doAddVxlan(args[0], vxlink, vxbr, untagged, vxinfo)
		},
	}
	addVxlan.Flags().BoolVarP(&learning, "learning", "l", false, "enable mac learning")
	addVxlan.Flags().Uint16VarP(&vxinfo.DstPort, "dstport", "d", 4789, "destination port")
	addVxlan.Flags().StringVarP(&local, "local", "t", "", "local tunnel IP")
	addVxlan.Flags().StringVarP(&vxlink, "link", "i", "", "parent link")
	addVxlan.Flags().StringVarP(&vxbr, "bridge", "b", "", "add to bridge")
	addVxlan.Flags().Uint16VarP(&untagged, "untagged", "u", 0, "set untagged bridge vlan")
	add.AddCommand(addVxlan)

	// addVeth
	var (
		vethNS string
		vebr   string
	)
	addVeth := &cobra.Command{
		Use:   "veth <name> <peer>",
		Short: "create a veth pair",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			doAddVeth(args[0], args[1], vethNS, vebr)
		},
	}
	addVeth.Flags().StringVarP(&vethNS, "namespace", "n", "", "network namespace")
	addVeth.Flags().StringVarP(&vebr, "bridge", "b", "", "add veth to bridge")
	add.AddCommand(addVeth)

	// set
	set := &cobra.Command{
		Use:   "set",
		Short: "add link properties",
	}
	link.AddCommand(set)

	untaggedCmd := &cobra.Command{
		Use:   "untagged <name> <vid>",
		Short: "set link untagged bridge vlan",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			vid, err := strconv.Atoi(args[1])
			if err != nil {
				log.Fatal(err)
			}
			doUntagged(args[0], vid, false)
		},
	}
	set.AddCommand(untaggedCmd)

	// unset
	unset := &cobra.Command{
		Use:   "unset",
		Short: "remove link properties",
	}
	link.AddCommand(unset)

	noUntaggedCmd := &cobra.Command{
		Use:   "untagged <name> <vid>",
		Short: "unset link untagged bridge vlan",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {
			vid, err := strconv.Atoi(args[1])
			if err != nil {
				log.Fatal(err)
			}
			doUntagged(args[0], vid, true)
		},
	}
	unset.AddCommand(noUntaggedCmd)

}

func doList(typ, bridge string) {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()

	links, err := rtnl.ReadLinks(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	linkT := rtnl.ParseLinkType(typ)
	if linkT != rtnl.UnspecLinkType {
		links = filter(typeFilter(linkT), links)
	}

	if bridge != "" {
		lnk, err := rtnl.GetLink(ctx, bridge)
		if err != nil {
			log.Fatal(err)
		}
		links = filter(bridgeFilter(uint32(lnk.Msg.Index)), links)
	}

	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
		"name",
		white("type"), //get colored offset correct for tab writer
		"mac",
		"master",
		"addrs",
		"props",
	)

	for _, link := range links {

		//read the bridge info
		link.Msg.Family = unix.AF_BRIDGE
		link.Read(ctx)

		showLink(ctx, link)
	}

	tw.Flush()

}

func doDelete(name string) {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()

	lnk, err := rtnl.GetLink(ctx, name)
	if err != nil {
		log.Fatal(err)
	}

	err = lnk.Del(ctx)
	if err != nil {
		log.Fatal(err)
	}

}

func doAddBridge(name string, info *rtnl.Bridge) {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()

	lnk := &rtnl.Link{
		Info: &rtnl.LinkInfo{
			Name:   name,
			Bridge: info,
		},
	}

	err = lnk.Add(ctx)
	if err != nil {
		log.Fatal(err)
	}

}

func doAddVxlan(name, parent, bridge string, untagged uint16, info *rtnl.Vxlan) {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()

	if parent != "" {
		p, err := rtnl.GetLink(ctx, parent)
		if err != nil {
			log.Fatal(err)
		}
		info.Link = uint32(p.Msg.Index)
	}

	lnk := &rtnl.Link{
		Info: &rtnl.LinkInfo{
			Name:  name,
			Vxlan: info,
		},
	}

	lnk.Info.Untagged = untagged

	if bridge != "" {
		b, err := rtnl.GetLink(ctx, bridge)
		if err != nil {
			log.Fatal(err)
		}
		lnk.Info.Master = uint32(b.Msg.Index)
	}

	err = lnk.Add(ctx)
	if err != nil {
		log.Fatal(err)
	}

}

func doAddVeth(a, b, namespace, bridge string) {

	var ctx *rtnl.Context
	var err error
	if namespace != "" {
		ctx, err = rtnl.OpenContext(namespace)
		if err != nil {
			log.Fatal(err)
		}
	} else {
		ctx, err = rtnl.OpenDefaultContext()
		if err != nil {
			log.Fatal(err)
		}
	}
	defer ctx.Close()

	lnk := &rtnl.Link{
		Info: &rtnl.LinkInfo{
			Ns:   uint32(ctx.Fd()),
			Name: a,
			Veth: &rtnl.Veth{Peer: b},
		},
	}

	if bridge != "" {
		b, err := rtnl.GetLink(ctx, bridge)
		if err != nil {
			log.Fatal(err)
		}
		lnk.Info.Master = uint32(b.Msg.Index)
	}

	err = lnk.Add(ctx)
	if err != nil {
		log.Fatal(err)
	}

}

func doUntagged(name string, vni int, unset bool) {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}

	lnk, err := rtnl.GetLink(ctx, name)
	if err != nil {
		log.Fatal(err)
	}
	lnk.Info.Untagged = uint16(vni)

	err = lnk.SetUntagged(ctx, unset)
	if err != nil {
		log.Fatal(err)
	}

}

// helpers ~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~~
type FilterFunc func(link *rtnl.Link) bool

func typeFilter(typ rtnl.LinkType) FilterFunc {
	return func(link *rtnl.Link) bool {
		return link.Info.Type() == typ
	}
}

func bridgeFilter(bridgeid uint32) FilterFunc {
	return func(link *rtnl.Link) bool {
		return link.Info.Master == bridgeid || link.Msg.Index == int32(bridgeid)
	}
}

func filter(filter FilterFunc, links []*rtnl.Link) []*rtnl.Link {

	result := []*rtnl.Link{}
	for _, l := range links {
		if filter(l) {
			result = append(result, l)
		}
	}
	return result

}

func showLink(ctx *rtnl.Context, l *rtnl.Link) {

	var typ string
	if l.Info.Type() == rtnl.PhysicalType {
		typ = green(l.Info.Type().String())
	} else {
		typ = blue(l.Info.Type().String())
	}

	master := ""
	if l.Info.Master != 0 {
		m, err := rtnl.GetLinkByIndex(ctx, int32(l.Info.Master))
		if err != nil {
			log.Fatal(err)
		}
		master = m.Info.Name
	}

	addrs, err := rtnl.ReadAddrs(ctx, &rtnl.Address{
		Msg: unix.IfAddrmsg{
			Index: uint32(l.Msg.Index),
		},
	})
	if err != nil {
		log.Fatal(err)
	}
	var addrList []string
	for _, x := range addrs {
		addrList = append(addrList, x.Info.Address.String())
	}

	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
		l.Info.Name,
		typ,
		l.Info.Address.String(),
		master,
		strings.Join(addrList, " "),
		props(l),
	)

}

func props(l *rtnl.Link) string {

	s := ""

	switch l.Info.Type() {
	case rtnl.BridgeType:
		s += bridgeProps(l) + " "
	}

	if l.Info.Master != 0 && l.Info.Untagged != 0 {
		s += fmt.Sprintf("untagged(%d)", l.Info.Untagged)
	}

	return s

}

func bridgeProps(l *rtnl.Link) string {

	if l.Info.Bridge == nil {
		return ""
	}

	if l.Info.Bridge.VlanAware {
		return "vlan-aware"
	}

	return ""

}
