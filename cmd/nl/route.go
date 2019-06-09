package main

import (
	"fmt"
	"log"
	"net"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"gitlab.com/mergetb/tech/rtnl"
)

func routeCommands(root *cobra.Command) {

	var (
		table int
	)
	route := &cobra.Command{
		Use:   "route",
		Short: "route command family",
	}
	route.PersistentFlags().IntVarP(&table, "table", "t", 0, "routing table")
	root.AddCommand(route)

	list := &cobra.Command{
		Use:   "list",
		Short: "list routes",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { routeList(table) },
	}
	route.AddCommand(list)

	var (
		src, gw, pref, iif, oif string
		prio                    int
	)
	routeFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVarP(&src, "source", "s", "", "route source")
		cmd.Flags().StringVarP(&gw, "gw", "g", "", "route gateway")
		cmd.Flags().StringVarP(&pref, "pref", "p", "", "route preferred source")
		cmd.Flags().StringVarP(&iif, "iif", "i", "", "route input interface")
		cmd.Flags().StringVarP(&oif, "oif", "o", "", "route output interface")
		cmd.Flags().IntVarP(&prio, "prio", "r", 0, "route priority")
	}

	add := &cobra.Command{
		Use:   "add <test>",
		Short: "add a route",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			ctx, err := rtnl.OpenDefaultContext()
			if err != nil {
				log.Fatal(err)
			}
			defer ctx.Close()

			route := makeRoute(args[0], src, gw, iif, oif, prio, table)
			err = route.Add(ctx)
			if err != nil {
				log.Fatal(err)
			}

		},
	}
	routeFlags(add)
	route.AddCommand(add)

	del := &cobra.Command{
		Use:   "del <dest>",
		Short: "delete a route",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			ctx, err := rtnl.OpenDefaultContext()
			if err != nil {
				log.Fatal(err)
			}
			defer ctx.Close()

			route := makeRoute(args[0], src, gw, iif, oif, prio, table)
			err = route.Del(ctx)
			if err != nil {
				log.Fatal(err)
			}

		},
	}
	routeFlags(del)
	route.AddCommand(del)

}

func routeList(table int) {

	if table == 0 {
		table = 254 // default table
	}

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()

	spec := &rtnl.Route{
		Hdr: unix.RtMsg{
			Family: unix.AF_INET,
		},
		Table: uint32(table),
	}
	routes, err := rtnl.ReadRoutes(ctx, spec)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		"src",
		"dest",
		"gateway",
		"prefsrc",
		"iif",
		"oif",
		"priority",
		"table",
		"metrics",
	)

	for _, route := range routes {

		if route.Table != uint32(table) {
			continue
		}

		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%d\t%d\t%d\n",
			routeLabel(route.Src, route.Hdr.Src_len),
			routeLabel(route.Dest, route.Hdr.Dst_len),
			targetLabel(route.Gateway),
			targetLabel(route.PrefSrc),
			ifLabel(ctx, route.Iif),
			ifLabel(ctx, route.Oif),
			route.Priority,
			route.Table,
			route.Metrics,
		)

	}

	tw.Flush()

}

func routeLabel(target net.IP, len uint8) string {

	label := "*"
	if target != nil {
		label = target.String()
		label += fmt.Sprintf("/%d", len)
	}
	return label

}

func ifLabel(ctx *rtnl.Context, ifx uint32) string {

	label := "*"
	if ifx != 0 {
		lnk, err := rtnl.GetLinkByIndex(ctx, int32(ifx))
		if err != nil {
			log.Fatal(err)
		}
		label = lnk.Info.Name
	}
	return label

}

func makeRoute(
	dest, src, gw, iif, oif string, prio, table int) *rtnl.Route {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()

	dst, dlen, err := parsePrefix(dest)
	if err != nil {
		log.Fatal("invalid destination")
	}

	var oi uint32 = 0
	if oif != "" {
		lnk, err := rtnl.GetLink(ctx, oif)
		if err != nil {
			log.Fatal(err)
		}
		oi = uint32(lnk.Msg.Index)
	}

	var ii uint32 = 0
	if iif != "" {
		lnk, err := rtnl.GetLink(ctx, iif)
		if err != nil {
			log.Fatal(err)
		}
		ii = uint32(lnk.Msg.Index)
	}

	return &rtnl.Route{
		Hdr: unix.RtMsg{
			Dst_len: dlen,
		},
		Dest:     dst,
		Src:      net.ParseIP(src),
		Gateway:  net.ParseIP(gw),
		Oif:      oi,
		Iif:      ii,
		Priority: uint32(prio),
		Table:    uint32(table),
	}

}
