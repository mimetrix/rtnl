package main

import (
	"log"

	"github.com/spf13/cobra"

	"gitlab.com/mergetb/tech/rtnl"
)

func macvlanCommands(root *cobra.Command) {

	macvlan := &cobra.Command{
		Use:   "macvlan",
		Short: "macvlan command family",
	}
	root.AddCommand(macvlan)

	add := &cobra.Command{
		Use:   "add <device> <name> <mode>",
		Short: "add macvlan",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {

			modMacvlan(args[0], args[1], args[2], false)

		},
	}
	macvlan.AddCommand(add)

	del := &cobra.Command{
		Use:   "del <device> <name> <table>",
		Short: "del macvlan",
		Args:  cobra.ExactArgs(3),
		Run: func(cmd *cobra.Command, args []string) {

			modMacvlan(args[0], args[1], args[2], true)

		},
	}
	macvlan.AddCommand(del)

}

func modMacvlan(dev, name, mode string, del bool) {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}

	m, err := rtnl.ParseMacvlanMode(mode)
	if err != nil {
		log.Fatal(err)
	}

	target, err := rtnl.GetLink(ctx, dev)
	if err != nil {
		log.Fatal(err)
	}

	link := rtnl.NewLink()
	link.Info.Name = name
	link.Info.Macvlan = &rtnl.Macvlan{
		Mode: m,
		Link: uint32(target.Msg.Index),
	}

	if del {
		err = link.Del(ctx)
	} else {
		err = link.Add(ctx)
	}

	if err != nil {
		log.Fatal(err)
	}

}
