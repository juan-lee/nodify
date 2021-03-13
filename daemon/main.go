package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"k8s.io/node-problem-detector/cmd/options"
	"k8s.io/node-problem-detector/pkg/exporters/k8sexporter"
	"k8s.io/node-problem-detector/pkg/types"
)

// +kubebuilder:rbac:groups="",resources=events;nodes,verbs=get;list;watch;create;update;delete;patch
// +kubebuilder:rbac:groups="",resources=nodes/status,verbs=get;update;patch

const interval = time.Second * 30

type scheduledEvents struct {
	DocumentIncarnation int     `json:"DocumentIncarnation"`
	Events              []event `json:"Events"`
}

type event struct {
	EventID      string   `json:"EventId"`
	EventType    string   `json:"EventType"`
	ResourceType string   `json:"ResourceType"`
	Resources    []string `json:"Resources"`
	EventStatus  string   `json:"EventStatus"`
	// TODO: parse datetime
	// NotBefore    time.Time `json:"NotBefore"`
	NotBefore   string `json:"NotBefore"`
	Description string `json:"Description"`
	EventSource string `json:"EventSource"`
}

func main() {
	npdo := options.NodeProblemDetectorOptions{
		EnableK8sExporter:          true,
		APIServerWaitTimeout:       time.Duration(5) * time.Minute,
		APIServerWaitInterval:      time.Duration(5) * time.Second,
		K8sExporterHeartbeatPeriod: time.Duration(5) * time.Minute,
		ServerAddress:              "127.0.0.1",
		ServerPort:                 20260,
		NodeName:                   os.Getenv("NODE_NAME"),
	}

	var previousEvents scheduledEvents
	exporter := k8sexporter.NewExporterOrDie(&npdo)
	for _ = range time.Tick(interval) {
		events, err := queryScheduledEvents()
		if err != nil {
			log.Fatal(err)
		}
		if events.DocumentIncarnation == previousEvents.DocumentIncarnation {
			continue
		}
		fmt.Printf("events: %+v\npreviousEvents: %+v\n", events, previousEvents)
		exporter.ExportProblems(convert(events))
		previousEvents = events
	}
}

func queryScheduledEvents() (scheduledEvents, error) {
	ctx := context.Background()
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/metadata/scheduledevents?api-version=2019-08-01", nil)
	if err != nil {
		return scheduledEvents{}, err
	}
	req.Header.Add("Metadata", "true")
	res, err := client.Do(req)
	if err != nil {
		return scheduledEvents{}, err
	}
	events, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return scheduledEvents{}, err
	}
	se := scheduledEvents{}
	if err := json.Unmarshal(events, &se); err != nil {
		return scheduledEvents{}, err
	}
	return se, nil
}

func convert(se scheduledEvents) *types.Status {
	status := types.Status{Source: "nodify"}
	for _, e := range se.Events {
		event := types.Event{
			Severity:  types.Warn,
			Timestamp: time.Now(),
			Reason:    e.EventType,
			Message:   e.Description,
		}
		status.Events = append(status.Events, event)
		condition := types.Condition{
			Type:       "MaintenanceScheduled",
			Status:     types.True,
			Transition: time.Now(),
			Reason:     e.EventType,
			Message:    e.Description,
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
