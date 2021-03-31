package metadata

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"
)

const (
	instanceURL        = "http://169.254.169.254/metadata/instance?api-version=2020-09-01"
	scheduledEventsURL = "http://169.254.169.254/metadata/scheduledevents?api-version=2019-08-01"
)

// Client for fetching Virtual Machine metadata and events.
type Client struct {
	self string
}

// ScheduledEvents schema for Virtual Machine maintenance response.
type ScheduledEvents struct {
	DocumentIncarnation int     `json:"DocumentIncarnation,omitempty"`
	Events              []Event `json:"Events,omitempty"`
}

type TimeRFC1123 struct{ time.Time }

// Event schema for Virtual Machine maintenance events.
type Event struct {
	EventID      string      `json:"EventId,omitempty"`
	EventType    string      `json:"EventType,omitempty"`
	ResourceType string      `json:"ResourceType,omitempty"`
	Resources    []string    `json:"Resources,omitempty"`
	EventStatus  string      `json:"EventStatus,omitempty"`
	NotBefore    TimeRFC1123 `json:"NotBefore,omitempty"`
	Description  string      `json:"Description,omitempty"`
	EventSource  string      `json:"EventSource,omitempty"`
}

func (t *TimeRFC1123) UnmarshalJSON(data []byte) error {
	dt := strings.Trim(string(data), "\"")
	if dt == "" {
		t.Time = time.Now()
		return nil
	}
	result, err := time.Parse(time.RFC1123, dt)
	if err != nil {
		return err
	}
	t.Time = result
	return nil
}

type instanceMetadata struct {
	Compute compute `json:"compute,omitempty"`
}

type compute struct {
	Name string `json:"name,omitempty"`
}

type scheduledEventsAck struct {
	StartRequests []startRequest `json:"StartRequests,omitempty"`
}

type startRequest struct {
	EventID string `json:"EventId,omitempty"`
}

// Scheduled returns ScheduledEvents containing a list of maintenance operations scheduled for the virtual machine.
func (c Client) Scheduled(ctx context.Context) (*ScheduledEvents, error) {
	if c.self == "" {
		im, err := c.instance(ctx)
		if err != nil {
			return nil, err
		}
		c.self = im.Compute.Name
	}
	se, err := scheduledEvents(ctx)
	if err != nil {
		return nil, err
	}
	var filtered []Event
	for n := range se.Events {
		for _, resource := range se.Events[n].Resources {
			if resource == c.self {
				filtered = append(filtered, se.Events[n])
				break
			}
		}
	}
	se.Events = filtered
	return se, nil
}

// AckAll acknowledges maintenance operations for execution.
func (c Client) AckAll(ctx context.Context, scheduled *ScheduledEvents) error {
	for n := range scheduled.Events {
		if err := ackEvent(ctx, &scheduled.Events[n]); err != nil {
			return err
		}
	}
	return nil
}

func (c Client) instance(ctx context.Context) (instanceMetadata, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", instanceURL, nil)
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

func scheduledEvents(ctx context.Context) (*ScheduledEvents, error) {
	req, err := http.NewRequestWithContext(ctx, "GET", scheduledEventsURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Add("Metadata", "true")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	events, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return nil, err
	}
	se := ScheduledEvents{}
	if err := json.Unmarshal(events, &se); err != nil {
		return nil, fmt.Errorf("cannot unmarshal json: %w\n%s", err, string(events))
	}
	return &se, nil
}

func ackEvent(ctx context.Context, event *Event) error {
	body, err := json.Marshal(scheduledEventsAck{[]startRequest{{EventID: event.EventID}}})
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, "POST", scheduledEventsURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Add("Metadata", "true")
	client := &http.Client{}
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	_, err = ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	return err
}
