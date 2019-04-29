package drift

import (
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/shogo82148/cfnutils/internal/subcommands/base"
)

// Drift detects drifts of the stack.
var Drift = &base.Command{
	UsageLine: "drift [-all] [-recursive] stack-name [stack-name]",
	Short:     "detect drifts of the stack",
}

var (
	flagAll       bool
	flagRecursive bool
)

func init() {
	Drift.Run = run
	Drift.Flag.BoolVar(&flagAll, "all", false, "detect drifts of all stacks")
	Drift.Flag.BoolVar(&flagAll, "a", false, "shorthand of -all")
	Drift.Flag.BoolVar(&flagRecursive, "recursive", false, "detect drifts of nested stacks recursively")
	Drift.Flag.BoolVar(&flagRecursive, "r", false, "shorthand of -recursive")
}

func run(cmd *base.Command, args []string) {
	detect(args)
}

func detect(stacks []string) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Println(err)
		base.SetExitStatus(1)
		base.Exit()
	}
	cfn := cloudformation.New(cfg)

	for _, stack := range stacks {
		req := cfn.DetectStackDriftRequest(&cloudformation.DetectStackDriftInput{
			StackName: aws.String(stack),
		})
		resp, err := req.Send()
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println(aws.StringValue(resp.StackDriftDetectionId))
	}
}
