package main

import (
	"flag"
	"log"
	"os"

	"github.com/shogo82148/cfnutils/internal/subcommands/base"
	"github.com/shogo82148/cfnutils/internal/subcommands/help"
	"github.com/shogo82148/cfnutils/internal/subcommands/tail"
)

func init() {
	base.Commands = []*base.Command{
		tail.Tail,
	}
}

func main() {
	flag.Parse()
	args := flag.Args()
	if len(args) < 1 {
		base.Usage()
	}

	if args[0] == "help" {
		help.Help.Run(help.Help, args[1:])
		return
	}

	for _, cmd := range base.Commands {
		if cmd.Name() == args[0] {
			cmd.Flag.Parse(args[1:])
			args = cmd.Flag.Args()
			cmd.Run(cmd, args)
			base.Exit()
			return
		}
	}
	log.Fatal("unknown command:", args[0])
}

func init() {
	base.Usage = mainUsage
}

func mainUsage() {
	help.PrintUsage(os.Stderr)
	os.Exit(2)
}
