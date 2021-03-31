package main

import (
	"context"
	"flag"
	"log"
	"os"
	"time"

	"daemon/metadata"

	"k8s.io/node-problem-detector/cmd/options"
	"k8s.io/node-problem-detector/pkg/exporters/k8sexporter"
	"k8s.io/node-problem-detector/pkg/types"
)

// +kubebuilder:rbac:groups="",resources=events;nodes,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="",resources=nodes/status,verbs=get;update;patch

const interval = time.Second * 30

func main() {
	flag.Parse()
	npdo := options.NodeProblemDetectorOptions{
		EnableK8sExporter:          true,
		APIServerWaitTimeout:       time.Duration(5) * time.Minute, // nolint: gomnd
		APIServerWaitInterval:      time.Duration(5) * time.Second, // nolint: gomnd
		K8sExporterHeartbeatPeriod: time.Duration(5) * time.Minute, // nolint: gomnd
		ServerAddress:              "127.0.0.1",
		ServerPort:                 20260,
		NodeName:                   os.Getenv("NODE_NAME"),
	}

	ctx := context.Background()
	client := metadata.Client{}

	acknowledged := false
	lastTransition := time.Now()
	previousEvents := &metadata.ScheduledEvents{}
	exporter := k8sexporter.NewExporterOrDie(&npdo)
	for range time.Tick(interval) {
		events, err := client.Scheduled(ctx)
		if err != nil {
			log.Fatalf("error getting scheduled events: %+v\n", err)
		}
		if events.DocumentIncarnation == previousEvents.DocumentIncarnation {
			if !lastTransition.IsZero() && time.Since(lastTransition) >= time.Minute && !acknowledged {
				log.Printf("AckAll: %+v", *events)
				if err := client.AckAll(ctx, events); err != nil {
					log.Printf("couldn't ack event: %v\n", err)
				}
				acknowledged = true
			}
			continue
		}
		log.Printf("events: %+v\npreviousEvents: %+v\n", events, previousEvents)
		exporter.ExportProblems(convert(events))
		acknowledged = false
		previousEvents = events
		lastTransition = time.Now()
	}
}

func convert(se *metadata.ScheduledEvents) *types.Status {
	status := types.Status{Source: "nodify"}
	for n := range se.Events {
		event := types.Event{
			Severity:  types.Warn,
			Timestamp: time.Now(),
			Reason:    se.Events[n].EventType,
			Message:   se.Events[n].Description,
		}
		status.Events = append(status.Events, event)
		condition := types.Condition{
			Type:       "MaintenanceScheduled",
			Status:     types.True,
			Transition: time.Now(),
			Reason:     se.Events[n].EventType,
			Message:    se.Events[n].Description,
		}
		status.Conditions = append(status.Conditions, condition)
	}
	if len(status.Conditions) == 0 {
		return noMaintenance()
	}
	return &status
}

func noMaintenance() *types.Status {
	return &types.Status{
		Source: "nodify",
		Conditions: []types.Condition{
			{
				Type:       "MaintenanceScheduled",
				Status:     types.False,
				Transition: time.Now(),
				Reason:     "None",
				Message:    "No maintenance scheduled.",
			},
		},
	}
}
