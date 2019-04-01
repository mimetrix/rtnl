package main

import (
	"log"
	"os"
	"text/tabwriter"

	"github.com/fatih/color"
	"github.com/spf13/cobra"
)

var tw = tabwriter.NewWriter(os.Stdout, 0, 0, 4, ' ', 0)
var white = color.New(color.FgWhite).SprintFunc()
var green = color.New(color.FgGreen).SprintFunc()
var blue = color.New(color.FgBlue).SprintFunc()
var cyan = color.New(color.FgCyan).SprintFunc()
var red = color.New(color.FgRed).SprintFunc()

func main() {

	log.SetFlags(0)

	cobra.EnablePrefixMatching = true

	root := &cobra.Command{
		Use:   "nl",
		Short: "netlink command line client",
	}

	linkCommands(root)
	ruleCommands(root)
	routeCommands(root)
	vrfCommands(root)
	macvlanCommands(root)

	root.Execute()

}
