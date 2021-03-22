package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
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

type instanceMetadata struct {
	Compute compute `json:"compute"`
}

type compute struct {
	Name string `json:"name"`
}

type scheduledEvents struct {
	Acknowleged         bool
	LastTransitionTime  time.Time
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

type scheduledEventsAck struct {
	StartRequests []startRequest `json:"StartRequests"`
}

type startRequest struct {
	EventID string `json:"EventId"`
}

func main() {
	flag.Parse()
	npdo := options.NodeProblemDetectorOptions{
		EnableK8sExporter:          true,
		APIServerWaitTimeout:       time.Duration(5) * time.Minute,
		APIServerWaitInterval:      time.Duration(5) * time.Second,
		K8sExporterHeartbeatPeriod: time.Duration(5) * time.Minute,
		ServerAddress:              "127.0.0.1",
		ServerPort:                 20260,
		NodeName:                   os.Getenv("NODE_NAME"),
	}

	im, err := getInstanceMetadata()
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Starting nodify for [%s]\n", im.Compute.Name)

	var previousEvents scheduledEvents
	exporter := k8sexporter.NewExporterOrDie(&npdo)
	for range time.Tick(interval) {
		events, err := getScheduledEventsForNode(im.Compute.Name)
		if err != nil {
			log.Fatal(err)
		}
		if events.DocumentIncarnation == previousEvents.DocumentIncarnation {
			if !previousEvents.LastTransitionTime.IsZero() &&
				time.Since(previousEvents.LastTransitionTime) >= time.Minute &&
				!previousEvents.Acknowleged {
				log.Println("ACKing events")
				for _, e := range previousEvents.Events {
					if err := ackScheduledEvent(e.EventID); err != nil {
						log.Printf("couldn't ack event: %v\n", err)
					}
				}
				previousEvents.Acknowleged = true
				log.Println("ACK'd events")
			}
			continue
		}
		log.Printf("events: %+v\npreviousEvents: %+v\n", events, previousEvents)
		events.LastTransitionTime = time.Now()
		exporter.ExportProblems(convert(events))
		previousEvents = events
	}
}

func getInstanceMetadata() (instanceMetadata, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/metadata/instance?api-version=2020-09-01", nil)
	if err != nil {
		return instanceMetadata{}, err
	}
	req.Header.Add("Metadata", "true")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return instanceMetadata{}, err
	}
	events, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return instanceMetadata{}, err
	}
	se := instanceMetadata{}
	if err := json.Unmarshal(events, &se); err != nil {
		return instanceMetadata{}, err
	}
	return se, nil
}

func getScheduledEventsForNode(node string) (scheduledEvents, error) {
	se, err := getScheduledEvents()
	if err != nil {
		return scheduledEvents{}, err
	}
	var filtered []event
	for _, event := range se.Events {
		for _, resource := range event.Resources {
			if resource == node {
				filtered = append(filtered, event)
				break
			}
		}
	}
	se.Events = filtered
	return se, nil
}

func getScheduledEvents() (scheduledEvents, error) {
	ctx := context.Background()
	req, err := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/metadata/scheduledevents?api-version=2019-08-01", nil)
	if err != nil {
		return scheduledEvents{}, err
	}
	req.Header.Add("Metadata", "true")
	client := &http.Client{}
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

func ackScheduledEvent(eventID string) error {
	ctx := context.Background()
	body, err := json.Marshal(scheduledEventsAck{[]startRequest{{EventID: eventID}}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", "http://169.254.169.254/metadata/scheduledevents?api-version=2019-08-01", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Add("Metadata", "true")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	events, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return err
	}
	fmt.Println(string(events))
	return nil
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
