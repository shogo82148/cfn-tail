package drift

import (
	"log"
	"strings"
	"time"

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
	// start to detect stack drift
	detectionIDs := make([]string, 0, len(stacks))
	for _, stack := range stacks {
		stack, err := canonicalStackName(cfn, stack)
		if err != nil {
			log.Println(err)
			continue
		}
		log.Println("start to detect stack drift: ", stack)
		req := cfn.DetectStackDriftRequest(&cloudformation.DetectStackDriftInput{
			StackName: aws.String(stack),
		})
		resp, err := req.Send()
		if err != nil {
			log.Println(err)
			continue
		}
		detectionIDs = append(detectionIDs, aws.StringValue(resp.StackDriftDetectionId))
	}

	// wait for completing
	for len(detectionIDs) > 0 {
		time.Sleep(5 * time.Second)
		inProgress := make([]string, 0, len(detectionIDs))
		for _, id := range detectionIDs {
			req := cfn.DescribeStackDriftDetectionStatusRequest(&cloudformation.DescribeStackDriftDetectionStatusInput{
				StackDriftDetectionId: aws.String(id),
			})
			resp, err := req.Send()
			if err != nil {
				// try to next time
				inProgress = append(inProgress, id)
				continue
			}
			switch resp.DetectionStatus {
			case cloudformation.StackDriftDetectionStatusDetectionComplete:
				// detection complete, show the result
				log.Printf("completed stack: %s, status: %s, drifted resource count: %d", aws.StringValue(resp.StackId), resp.StackDriftStatus, aws.Int64Value(resp.DriftedStackResourceCount))
			case cloudformation.StackDriftDetectionStatusDetectionFailed:
				// detection failed
				log.Printf("failed stack:%s, reason:%s", aws.StringValue(resp.StackId), aws.StringValue(resp.DetectionStatusReason))
			case cloudformation.StackDriftDetectionStatusDetectionInProgress:
				inProgress = append(inProgress, id)
			}
		}
		detectionIDs = inProgress
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

func canonicalStackName(cfn cloudformationiface.CloudFormationAPI, stackName string) (string, error) {
	if !strings.HasPrefix(stackName, "arn:") {
		req := cfn.DescribeStacksRequest(&cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		})
		resp, err := req.Send()
		if err != nil {
			return "", err
		}
		return aws.StringValue(resp.Stacks[0].StackId), nil
	}
	return stackName, nil
}
