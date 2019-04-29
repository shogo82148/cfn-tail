package drift

import (
	"log"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/cloudformationiface"
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
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Println(err)
		base.SetExitStatus(1)
		base.Exit()
	}
	cfn := cloudformation.New(cfg)

	if flagAll {
		stacks := []string{}
		req := cfn.ListStacksRequest(&cloudformation.ListStacksInput{
			StackStatusFilter: []cloudformation.StackStatus{
				cloudformation.StackStatusCreateComplete,
				cloudformation.StackStatusUpdateComplete,
				cloudformation.StackStatusUpdateRollbackComplete,
				cloudformation.StackStatusRollbackComplete,
			},
		})
		p := req.Paginate()
		for p.Next() {
			resp := p.CurrentPage()
			for _, stack := range resp.StackSummaries {
				stacks = append(stacks, aws.StringValue(stack.StackId))
			}
		}
		if err := p.Err(); err != nil {
			log.Println(err)
			base.SetExitStatus(1)
			base.Exit()
		}
		detect(cfn, stacks)
		return
	}

	if flagRecursive {
		stacks := make([]string, len(args))
		copy(stacks, args)
		for _, stack := range args {
			children, err := getChildren(cfn, stack)
			if err != nil {
				log.Println(err)
				base.SetExitStatus(1)
				base.Exit()
			}
			stacks = append(stacks, children...)
		}
		detect(cfn, stacks)
		return
	}

	detect(cfn, args)
}

func detect(cfn cloudformationiface.CloudFormationAPI, stacks []string) {
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

func getChildren(cfn cloudformationiface.CloudFormationAPI, stack string) ([]string, error) {
	req := cfn.ListStackResourcesRequest(&cloudformation.ListStackResourcesInput{
		StackName: aws.String(stack),
	})
	p := req.Paginate()
	children := []string{}
	for p.Next() {
		resp := p.CurrentPage()
		for _, rsc := range resp.StackResourceSummaries {
			if aws.StringValue(rsc.ResourceType) != "AWS::CloudFormation::Stack" {
				continue
			}
			children = append(children, aws.StringValue(rsc.PhysicalResourceId))
		}
	}
	if err := p.Err(); err != nil {
		return nil, err
	}
	grandchildren := []string{}
	for _, stack := range children {
		res, err := getChildren(cfn, stack)
		if err != nil {
			return nil, err
		}
		grandchildren = append(grandchildren, res...)
	}
	return append(children, grandchildren...), nil
}
