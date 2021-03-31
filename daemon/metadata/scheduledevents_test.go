package metadata

import (
	"encoding/json"
	"testing"
)

func TestEmptyNotBefore(t *testing.T) {
	scheduledEventsJSON := []byte(`
{
	"DocumentIncarnation":208,
	"Events":[{
			"EventId":"D3D9DFEC-1DCA-4B49-97AA-780E02F45DFE",
			"EventStatus":"Started",
			"EventType":"Freeze",
			"ResourceType":"VirtualMachine",
			"Resources":["aks-nodepool1-21922338-vmss_47"],
			"NotBefore":"",
			"Description":"Host server is undergoing maintenance.",
			"EventSource":"Platform"
	}]}
`)
	se := ScheduledEvents{}
	if err := json.Unmarshal(scheduledEventsJSON, &se); err != nil {
		t.Error(err)
	}
}

func TestNotBefore(t *testing.T) {
	scheduledEventsJSON := []byte(`
{
	"DocumentIncarnation":213,
	"Events":[{
		"EventId":"FA298C74-AE95-4154-8EBF-303EED382DB6",
		"EventStatus":"Scheduled",
		"EventType":"Reboot",
		"ResourceType":"VirtualMachine",
		"Resources":["aks-nodepool1-21922338-vmss_39"],
		"NotBefore":"Tue, 30 Mar 2021 13:39:24 GMT",
		"Description":"Virtual machine is going to be restarted as requested by authorized user.",
		"EventSource":"User"}
	]}
`)
	se := ScheduledEvents{}
	if err := json.Unmarshal(scheduledEventsJSON, &se); err != nil {
		t.Error(err)
	}
}
