package base

import (
	"flag"
	"os"
	"strings"
	"sync"
)

// Command is a base struct of commands.
type Command struct {
	Run       func(cmd *Command, args []string)
	Flag      flag.FlagSet
	UsageLine string
	Short     string
}

// Commands are available commands.
var Commands []*Command

// Name returns the name of the command
func (c *Command) Name() string {
	name := c.UsageLine
	i := strings.Index(name, " ")
	if i >= 0 {
		name = name[:i]
	}
	return name
}

// Exit exits the program.
func Exit() {
	os.Exit(exitStatus)
}

var exitStatus = 0
var exitMu sync.Mutex

// SetExitStatus sets the exit code.
func SetExitStatus(n int) {
	exitMu.Lock()
	if exitStatus < n {
		exitStatus = n
	}
	exitMu.Unlock()
}

// Usage is the usage-reporting function, filled in by package main
// but here for reference by other packages.
var Usage func()
