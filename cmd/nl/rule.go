package main

import (
	"fmt"
	"log"
	"net"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/sys/unix"

	"gitlab.com/mergetb/tech/rtnl"
)

func ruleCommands(root *cobra.Command) {

	rule := &cobra.Command{
		Use:   "rule",
		Short: "rule command family",
	}
	root.AddCommand(rule)

	list := &cobra.Command{
		Use:   "list",
		Short: "list rules",
		Args:  cobra.NoArgs,
		Run:   func(cmd *cobra.Command, args []string) { ruleList() },
	}
	rule.AddCommand(list)

	var (
		src, dst string
		iif, oif string
		mrk      int
	)
	ruleFlags := func(cmd *cobra.Command) {
		cmd.Flags().StringVarP(&src, "source", "s", "", "source prefix")
		cmd.Flags().StringVarP(&dst, "dest", "d", "", "destination prefix")
		cmd.Flags().StringVarP(&iif, "iif", "i", "", "incoming interface")
		cmd.Flags().StringVarP(&oif, "oif", "o", "", "outgoing interface")
		cmd.Flags().IntVarP(&mrk, "mark", "m", 0, "packet forwarding mark")
	}

	add := &cobra.Command{
		Use:   "add",
		Short: "add rule <priority> <table>",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {

			ctx, err := rtnl.OpenDefaultContext()
			if err != nil {
				log.Fatal(err)
			}
			defer ctx.Close()

			rule := makeRule(args[0], args[1], src, dst, iif, oif, mrk)
			err = rule.Add(ctx)
			if err != nil {
				log.Fatal(err)
			}

		},
	}
	ruleFlags(add)
	rule.AddCommand(add)

	del := &cobra.Command{
		Use:   "del",
		Short: "del rule <table>",
		Args:  cobra.ExactArgs(1),
		Run: func(cmd *cobra.Command, args []string) {

			ctx, err := rtnl.OpenDefaultContext()
			if err != nil {
				log.Fatal(err)
			}
			defer ctx.Close()

			rule := makeRule("0", args[0], src, dst, iif, oif, mrk)
			err = rule.Del(ctx)
			if err != nil {
				log.Fatal(err)
			}

		},
	}
	ruleFlags(del)
	rule.AddCommand(del)

}

func makeRule(priority, table, src, dest, iif, oif string, mrk int) *rtnl.Rule {

	prio, err := strconv.Atoi(priority)
	if err != nil {
		log.Fatal("priority must be an integer")
	}

	tbl, err := strconv.Atoi(table)
	if err != nil {
		log.Fatal("table must be an integer")
	}

	rule := &rtnl.Rule{
		Priority: uint32(prio),
		Iif:      iif,
		Oif:      oif,
		Fwmark:   uint32(mrk),
		Table:    uint32(tbl),
		Fib: rtnl.Fib{
			Family: unix.AF_INET,
		},
	}

	srcip, srclen, err := parsePrefix(src)
	if err != nil {
		log.Fatal("invalid source prefix")
	}
	rule.Src = srcip
	rule.Fib.SrcLen = srclen

	destip, destlen, err := parsePrefix(src)
	if err != nil {
		log.Fatal("invalid destination prefix")
	}
	rule.Dest = destip
	rule.Fib.DstLen = destlen

	return rule

}

func parsePrefix(pfx string) (net.IP, uint8, error) {

	if pfx == "" {
		return nil, 0, nil
	}

	ip, nw, err := net.ParseCIDR(pfx)
	if err == nil {
		ones, _ := nw.Mask.Size()
		return ip, uint8(ones), nil
	}

	ip = net.ParseIP(pfx)
	if ip != nil {
		return ip, 32, nil
	}

	return nil, 0, err

}

func ruleList() {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}
	defer ctx.Close()

	rules, err := rtnl.ReadRules(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
		"priority",
		"src",
		"dest",
		"iif",
		"oif",
		"fwmark",
		"table",
	)

	for _, rule := range rules {

		fmt.Fprintf(tw, "%d\t%s\t%s\t%s\t%s\t%s\t%d\n",
			rule.Priority,
			targetLabel(rule.Src),
			targetLabel(rule.Dest),
			rule.Iif,
			rule.Oif,
			markLabel(rule.Fwmark),
			rule.Table,
		)
	}

	tw.Flush()

}

func targetLabel(target net.IP) string {

	label := "*"
	if target != nil {
		label = target.String()
	}
	return label

}

func markLabel(mrk uint32) string {

	if mrk == 0 {
		return ""
	}
	return fmt.Sprintf("%d", mrk)

}
