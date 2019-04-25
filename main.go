package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/aws/external"
	"github.com/aws/aws-sdk-go-v2/service/cloudformation"
	"github.com/mattn/go-runewidth"
)

func main() {
	cfg, err := external.LoadDefaultAWSConfig()
	if err != nil {
		log.Fatal(err)
	}

	t := newTail(cfg)
	t.Start(context.Background(), os.Args[1])
	for e := range t.Events() {
		fmt.Println(formatEvent(e))
	}
}

func formatEvent(event cloudformation.StackEvent) string {
	type column struct {
		color color
		value string
		width int
	}
	columns := []column{
		{
			value: aws.TimeValue(event.Timestamp).Format(time.RFC3339),
			width: 20,
		},
		{
			color: yellow,
			value: aws.StringValue(event.StackName),
			width: 20,
		},
		{
			color: yellow,
			value: aws.StringValue(event.LogicalResourceId),
			width: 20,
		},
		{
			color: black | bright,
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
		buf.WriteString(colorize(c.value, c.color))
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

func statusColor(status string) color {
	switch status {
	case "CREATE_COMPLETE", "UPDATE_COMPLETE", "UPDATE_ROLLBACK_COMPLETE", "ROLLBACK_COMPLETE", "DELETE_COMPLETE":
		// positive events
		return green
	case "CREATE_FAILED", "DELETE_FAILED", "ROLLBACK_FAILED", "UPDATE_FAILED", "UPDATE_ROLLBACK_FAILED":
		// negative events
		return red
	case "CREATE_IN_PROGRESS", "DELETE_IN_PROGRESS", "UPDATE_IN_PROGRESS", "ROLLBACK_IN_PROGRESS", "UPDATE_COMPLETE_CLEANUP_IN_PROGRESS", "UPDATE_ROLLBACK_IN_PROGRESS", "UPDATE_ROLLBACK_COMPLETE_CLEANUP_IN_PROGRESS":
		return black | bright
	}
	return 0
}
