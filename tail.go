package main

import (
	"log"
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
}

func newTail(cfg aws.Config) *tail {
	return &tail{
		cfn: cloudformation.New(cfg),
		ch:  make(chan cloudformation.StackEvent, 8),
	}
}

func (t *tail) Events() <-chan cloudformation.StackEvent {
	return t.ch
}

func (t *tail) Start(stackName string) {
	t.start(stackName)
	go func() {
		t.wg.Wait()
		close(t.ch)
	}()
}

func (t *tail) start(stackName string) {
	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		req := t.cfn.DescribeStackEventsRequest(&cloudformation.DescribeStackEventsInput{
			StackName: aws.String(stackName),
		})
		resp, err := req.Send()
		if err != nil {
			log.Println(err)
			return
		}
		latestEvent := resp.StackEvents[0]

		for {
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
				}
			}
			for i := range events {
				t.ch <- events[len(events)-i-1]
			}
			if len(events) > 0 {
				latestEvent = events[0]

				// action finished?
				if aws.StringValue(latestEvent.PhysicalResourceId) == aws.StringValue(latestEvent.StackId) {
					switch string(latestEvent.ResourceStatus) {
					case "CREATE_FAILED", "CREATE_COMPLETE", // create finished.
						"ROLLBACK_FAILED", "ROLLBACK_COMPLETE", // rollback finished.
						"DELETE_FAILED", "DELETE_COMPLETE", // delete finished.
						"UPDATE_COMPLETE", "UPDATE_ROLLBACK_FAILED", "UPDATE_ROLLBACK_COMPLETE": // update finished.
						return
					}
				}
			}
			time.Sleep(time.Second)
		}
	}()
}
