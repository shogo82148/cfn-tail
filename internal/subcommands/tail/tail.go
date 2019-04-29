package tail

import (
	"context"
	"fmt"
	"log"
	"math/rand"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation/cloudformationiface"
	"github.com/mattn/go-runewidth"
	"github.com/shogo82148/cfnutils/internal/color"
	"github.com/shogo82148/cfnutils/internal/subcommands/base"
)

// Tail tails the events of CloudForamtion Stack.
var Tail = &base.Command{
	UsageLine: "tail stack-name",
	Short:     "tails the events of CloudForamtion Stack",
}

func init() {
	Tail.Run = run
}

func run(cmd *base.Command, args []string) {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Println(err)
		base.SetExitStatus(1)
		base.Exit()
	}
	if len(args) < 1 {
		log.Println("cfnutil tail STACK_NAME")
		base.SetExitStatus(1)
		base.Exit()
	}

	t := newTail(cfg)
	t.Start(context.Background(), args[0])
	for e := range t.Events() {
		fmt.Println(formatEvent(e))
	}
}

func formatEvent(event cloudformation.StackEvent) string {
	type column struct {
		color color.Color
		value string
		width int
	}
	columns := []column{
		{
			value: aws.TimeValue(event.Timestamp).Format(time.RFC3339),
			width: 20,
		},
		{
			color: color.Yellow,
			value: aws.StringValue(event.StackName),
			width: 20,
		},
		{
			color: color.Yellow,
			value: aws.StringValue(event.LogicalResourceId),
			width: 20,
		},
		{
			color: color.Black | color.Bright,
			value: aws.StringValue(event.ResourceType),
			width: 20,
		},
		{
			color: statusColor(string(event.ResourceStatus)),
			value: string(event.ResourceStatus),
			width: 30,
		},
		{
			value: aws.StringValue(event.ResourceStatusReason),
		},
	}

	var buf strings.Builder
	for _, c := range columns {
		buf.WriteString(color.Colorize(c.value, c.color))
		if c.width > 0 {
			padding := c.width - runewidth.StringWidth(c.value)
			for i := 0; i < padding; i++ {
				buf.WriteRune(' ')
			}
			buf.WriteString("  ")
		}
	}

	return buf.String()
}

func statusColor(status string) color.Color {
	switch status {
	case "CREATE_COMPLETE", "UPDATE_COMPLETE", "UPDATE_ROLLBACK_COMPLETE", "ROLLBACK_COMPLETE", "DELETE_COMPLETE":
		// positive events
		return color.Green
	case "CREATE_FAILED", "DELETE_FAILED", "ROLLBACK_FAILED", "UPDATE_FAILED", "UPDATE_ROLLBACK_FAILED":
		// negative events
		return color.Red
	case "CREATE_IN_PROGRESS", "DELETE_IN_PROGRESS", "UPDATE_IN_PROGRESS", "ROLLBACK_IN_PROGRESS", "UPDATE_COMPLETE_CLEANUP_IN_PROGRESS", "UPDATE_ROLLBACK_IN_PROGRESS", "UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS":
		return color.Black | color.Bright
	}
	return 0
}

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
		ctx, cancel := context.WithCancel(ctx)
		defer cancel()

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
						if aws.StringValue(e.PhysicalResourceId) != "" {
							// follow nested stack
							t.start(ctx, aws.StringValue(e.PhysicalResourceId))
						}
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
