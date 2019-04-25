package main

import (
	"context"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/cloudformationiface"
)

type tail struct {
	cfn cloudformationiface.CloudFormationAPI
	ch  chan cloudformation.StackEvent
	wg  sync.WaitGroup

	mu     sync.RWMutex
	stacks map[string]struct{}
}

func newTail(cfg aws.Config) *tail {
	return &tail{
		cfn:    cloudformation.New(cfg),
		ch:     make(chan cloudformation.StackEvent, 8),
		stacks: make(map[string]struct{}),
	}
}

func (t *tail) Events() <-chan cloudformation.StackEvent {
	return t.ch
}

func (t *tail) Start(ctx context.Context, stackName string) {
	t.start(ctx, stackName)
	go func() {
		t.wg.Wait()
		close(t.ch)
	}()
}

func (t *tail) start(ctx context.Context, stackName string) {
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	// canonical stack names to ARN
	if !strings.HasPrefix(stackName, "arn:") {
		req := t.cfn.DescribeStacksRequest(&cloudformation.DescribeStacksInput{
			StackName: aws.String(stackName),
		})
		req.SetContext(ctx)
		resp, err := req.Send()
		if err != nil {
			log.Println(err)
			return
		}
		stackName = aws.StringValue(resp.Stacks[0].StackId)
	}

	t.mu.Lock()
	if _, ok := t.stacks[stackName]; ok {
		// already tailing, skip.
		t.mu.Unlock()
		return
	}
	t.stacks[stackName] = struct{}{}
	t.mu.Unlock()

	t.wg.Add(1)
	go func() {
		defer func() {
			t.mu.Lock()
			delete(t.stacks, stackName)
			t.mu.Unlock()
			t.wg.Done()
		}()

		req := t.cfn.DescribeStackEventsRequest(&cloudformation.DescribeStackEventsInput{
			StackName: aws.String(stackName),
		})
		req.SetContext(ctx)
		resp, err := req.Send()
		if err != nil {
			// ignore canceling errors
			if ctx.Err() == nil {
				log.Println(err)
			}
			return
		}
		latestEvent := resp.StackEvents[0]

		for {
			if err := sleepWithContext(ctx, 2*time.Second+time.Duration(rand.Float64()*float64(time.Second))); err != nil {
				return
			}
			events := make([]cloudformation.StackEvent, 0, 10)
			req := t.cfn.DescribeStackEventsRequest(&cloudformation.DescribeStackEventsInput{
				StackName: aws.String(stackName),
			})
			p := req.Paginate()
		PAGENATE:
			for p.Next() {
				page := p.CurrentPage()
				for _, e := range page.StackEvents {
					if aws.StringValue(e.EventId) == aws.StringValue(latestEvent.EventId) {
						break PAGENATE
					}
					events = append(events, e)

					if aws.StringValue(e.ResourceType) == "AWS::CloudFormation::Stack" &&
						(e.ResourceStatus == "CREATE_IN_PROGRESS" ||
							e.ResourceStatus == "UPDATE_IN_PROGRESS" ||
							e.ResourceStatus == "DELETE_IN_PROGRESS") {
						// follow nested stack
						t.start(ctx, aws.StringValue(e.PhysicalResourceId))
					}
				}
			}
			for i := range events {
				t.ch <- events[len(events)-i-1]
			}
			if len(events) > 0 {
				latestEvent = events[0]

				// action finished?
				if aws.StringValue(latestEvent.PhysicalResourceId) == aws.StringValue(latestEvent.StackId) {
					switch latestEvent.ResourceStatus {
					case "CREATE_FAILED", "CREATE_COMPLETE", // create finished.
						"ROLLBACK_FAILED", "ROLLBACK_COMPLETE", // rollback finished.
						"DELETE_FAILED", "DELETE_COMPLETE", // delete finished.
						"UPDATE_COMPLETE", "UPDATE_ROLLBACK_FAILED", "UPDATE_ROLLBACK_COMPLETE": // update finished.
						return
					}
				}
			}
		}
	}()
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
	case <-ctx.Done():
		return ctx.Err()
	}
	return nil
}
