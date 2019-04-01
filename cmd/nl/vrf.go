package main

import (
	"log"
	"strconv"

	"github.com/spf13/cobra"

	"gitlab.com/mergetb/tech/rtnl"
)

func vrfCommands(root *cobra.Command) {

	vrf := &cobra.Command{
		Use:   "vrf",
		Short: "vrf command family",
	}
	root.AddCommand(vrf)

	add := &cobra.Command{
		Use:   "add <name> <table>",
		Short: "add vrf",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {

			table, err := strconv.Atoi(args[1])
			if err != nil {
				log.Fatal("table must be integer")
			}

			modVrf(args[0], table, false)

		},
	}
	vrf.AddCommand(add)

	del := &cobra.Command{
		Use:   "del <name> <table>",
		Short: "del vrf",
		Args:  cobra.ExactArgs(2),
		Run: func(cmd *cobra.Command, args []string) {

			table, err := strconv.Atoi(args[1])
			if err != nil {
				log.Fatal("table must be integer")
			}

			modVrf(args[0], table, true)

		},
	}
	vrf.AddCommand(del)

}

func modVrf(name string, table int, del bool) {

	ctx, err := rtnl.OpenDefaultContext()
	if err != nil {
		log.Fatal(err)
	}

	link := rtnl.NewLink()
	link.Info.Name = name
	link.Info.Vrf = &rtnl.Vrf{
		Table: uint32(table),
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
