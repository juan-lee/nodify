package main

import (
	"context"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"time"
)

const interval = time.Second * 30

func main() {
	for t := range time.Tick(interval) {
		if err := queryScheduledEvents(t); err != nil {
			log.Fatal(err)
		}
	}
}

func queryScheduledEvents(t time.Time) error {
	ctx := context.Background()
	client := &http.Client{}
	req, err := http.NewRequestWithContext(ctx, "GET", "http://169.254.169.254/metadata/scheduledevents?api-version=2019-08-01", nil)
	if err != nil {
		return err
	}
	req.Header.Add("Metadata", "true")
	res, err := client.Do(req)
	if err != nil {
		return err
	}
	events, err := ioutil.ReadAll(res.Body)
	defer res.Body.Close()
	if err != nil {
		return err
	}
	fmt.Printf("%s: %s\n", t, events)
	return nil
}
