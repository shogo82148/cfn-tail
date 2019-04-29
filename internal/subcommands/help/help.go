package help

import (
	"bufio"
	"io"
	"os"
	"text/template"

	"github.com/shogo82148/cfnutils/internal/subcommands/base"
)

// Help shows help message.
var Help = &base.Command{
	UsageLine: "help show help message",
}

func init() {
	Help.Run = run
}

func run(cmd *base.Command, args []string) {
	if len(args) == 0 {
		PrintUsage(os.Stdout)
		return
	}
}

var usageTemplate = `cfnutil: CloudFormation utils
Usage:
	cfnutil command [arguments]
The commands are:
{{range .}}
	{{.Name | printf "%-11s"}} {{.Short}}{{end}}
`

// PrintUsage shows the usage.
func PrintUsage(w io.Writer) {
	bw := bufio.NewWriter(w)
	t := template.New("top")
	template.Must(t.Parse(usageTemplate))
	if err := t.Execute(bw, base.Commands); err != nil {
		panic(err)
	}
	bw.Flush()
}
