package drift

import (
	"log"
	"math/rand"
	"strings"
	"sync"
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
	var wg sync.WaitGroup
	for _, stack := range stacks {
		stack := stack
		wg.Add(1)
		go func() {
			defer wg.Done()
			detectStack(cfn, stack)
		}()
		time.Sleep(time.Second)
	}
	wg.Wait()
}

func detectStack(cfn cloudformationiface.CloudFormationAPI, stack string) {
	stack, err := canonicalStackName(cfn, stack)
	if err != nil {
		log.Println(err)
		return
	}
	log.Println("start to detect stack drift: ", stack)
	req := cfn.DetectStackDriftRequest(&cloudformation.DetectStackDriftInput{
		StackName: aws.String(stack),
	})
	resp, err := req.Send()
	if err != nil {
		log.Println("failed to detect stack drift: ", stack, " ,", err)
		return
	}
	detectionID := aws.StringValue(resp.StackDriftDetectionId)

	for {
		time.Sleep(5*time.Second + time.Duration(rand.Float64()*float64(time.Second)))
		req := cfn.DescribeStackDriftDetectionStatusRequest(&cloudformation.DescribeStackDriftDetectionStatusInput{
			StackDriftDetectionId: aws.String(detectionID),
		})
		resp, err := req.Send()
		if err != nil {
			// try to next time
			continue
		}
		switch resp.DetectionStatus {
		case cloudformation.StackDriftDetectionStatusDetectionComplete:
			// detection complete, show the result
			log.Printf("completed status: %s, stack: %s, drifted resource count: %d", resp.StackDriftStatus, aws.StringValue(resp.StackId), aws.Int64Value(resp.DriftedStackResourceCount))
			return
		case cloudformation.StackDriftDetectionStatusDetectionFailed:
			// detection failed
			log.Printf("failed stack:%s, reason:%s", aws.StringValue(resp.StackId), aws.StringValue(resp.DetectionStatusReason))
			return
		case cloudformation.StackDriftDetectionStatusDetectionInProgress:
		}
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
