package main

import (
	"bytes"
	"crypto/tls"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
)

// Command line flags
var (
	serverURL   string
	eventType   string
	deviceID    string
	channelID   string
	zone        string
	username    string
	password    string
	insecure    bool
	repeatCount int
	interval    int
)

// VivotekEvent matches the structure expected by the API
type VivotekEvent struct {
	EventType    string                 `json:"eventType"`
	EventTime    time.Time              `json:"eventTime"`
	DeviceID     string                 `json:"deviceId"`
	ChannelID    string                 `json:"channelId"`
	EventDetails map[string]interface{} `json:"eventDetails"`
}

func init() {
	// Define command line flags
	flag.StringVar(&serverURL, "url", "http://localhost:8080/event", "API server URL")
	flag.StringVar(&eventType, "type", "MotionDetection", "Event type (MotionDetection, VideoLoss, DeviceConnection)")
	flag.StringVar(&deviceID, "device", "NVR12345", "Device ID")
	flag.StringVar(&channelID, "channel", "Camera01", "Channel ID")
	flag.StringVar(&zone, "zone", "Zone1", "Detection zone (for motion events)")
	flag.StringVar(&username, "user", "", "Basic auth username")
	flag.StringVar(&password, "pass", "", "Basic auth password")
	flag.BoolVar(&insecure, "insecure", false, "Skip TLS verification")
	flag.IntVar(&repeatCount, "repeat", 1, "Number of events to send")
	flag.IntVar(&interval, "interval", 5, "Interval between events in seconds")
}

func main() {
	flag.Parse()

	// Print client configuration
	fmt.Println("Vivotek API Test Client")
	fmt.Println("=======================")
	fmt.Printf("Server URL: %s\n", serverURL)
	fmt.Printf("Event Type: %s\n", eventType)
	fmt.Printf("Device ID: %s\n", deviceID)
	fmt.Printf("Channel ID: %s\n", channelID)
	if eventType == "MotionDetection" {
		fmt.Printf("Zone: %s\n", zone)
	}
	fmt.Printf("Auth: %v\n", username != "")
	fmt.Printf("Sending %d events with %d second intervals\n", repeatCount, interval)
	fmt.Println("=======================")

	// Configure HTTP client with optional TLS settings
	httpClient := &http.Client{}
	if insecure {
		httpClient.Transport = &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		}
	}

	// Send events
	for i := 1; i <= repeatCount; i++ {
		if i > 1 {
			fmt.Printf("Waiting %d seconds...\n", interval)
			time.Sleep(time.Duration(interval) * time.Second)
		}

		fmt.Printf("Sending event %d of %d\n", i, repeatCount)
		err := sendEvent(httpClient)
		if err != nil {
			log.Fatalf("Failed to send event: %v", err)
		}
	}

	fmt.Println("All events sent successfully!")
}

func sendEvent(client *http.Client) error {
	// Create event details based on event type
	eventDetails := make(map[string]interface{})

	switch eventType {
	case "MotionDetection":
		eventDetails["zoneId"] = zone
		eventDetails["confidence"] = 85
	case "VideoLoss":
		eventDetails["duration"] = 30
		eventDetails["cause"] = "cable disconnected"
	case "DeviceConnection":
		eventDetails["status"] = "disconnected"
		eventDetails["reason"] = "network failure"
	}

	// Create the event payload
	event := VivotekEvent{
		EventType:    eventType,
		EventTime:    time.Now(),
		DeviceID:     deviceID,
		ChannelID:    channelID,
		EventDetails: eventDetails,
	}

	// Marshal to JSON
	payload, err := json.MarshalIndent(event, "", "  ")
	if err != nil {
		return fmt.Errorf("error creating JSON payload: %v", err)
	}

	// Print the payload for debugging
	fmt.Println("Event payload:")
	fmt.Println(string(payload))

	// Create the request
	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(payload))
	if err != nil {
		return fmt.Errorf("error creating request: %v", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add basic auth if credentials were provided
	if username != "" {
		req.SetBasicAuth(username, password)
	}

	// Send the request
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending request: %v", err)
	}
	defer resp.Body.Close()

	// Read the response
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response: %v", err)
	}

	// Check the status code
	if resp.StatusCode >= 400 {
		return fmt.Errorf("server returned error: %d - %s", resp.StatusCode, string(respBody))
	}

	// Print the response
	fmt.Printf("Response status: %d\n", resp.StatusCode)
	fmt.Printf("Response: %s\n", string(respBody))

	return nil
}

// EventGenerator returns a function that creates custom events
func EventGenerator() func(string, string, string) VivotekEvent {
	return func(eventType, deviceID, channelID string) VivotekEvent {
		eventDetails := make(map[string]interface{})
		switch eventType {
		case "MotionDetection":
			eventDetails["zoneId"] = "Zone1"
			eventDetails["confidence"] = 85
		case "VideoLoss":
			eventDetails["duration"] = 30
		case "DeviceConnection":
			eventDetails["status"] = "connected"
		}

		return VivotekEvent{
			EventType:    eventType,
			EventTime:    time.Now(),
			DeviceID:     deviceID,
			ChannelID:    channelID,
			EventDetails: eventDetails,
		}
	}
}
