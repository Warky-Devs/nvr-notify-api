package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

// Command line flags
var (
	serverURL     string
	username      string
	password      string
	concurrency   int
	scenarioFile  string
	outputResults bool
)

// TestScenario represents a collection of test events to send
type TestScenario struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Events      []EventConfig `json:"events"`
}

// EventConfig represents a single event configuration
type EventConfig struct {
	EventType    string                 `json:"eventType"`
	DeviceID     string                 `json:"deviceId"`
	ChannelID    string                 `json:"channelId"`
	DelaySeconds int                    `json:"delaySeconds"`
	EventDetails map[string]interface{} `json:"eventDetails"`
}

// VivotekEvent matches the structure expected by the API
type VivotekEvent struct {
	EventType    string                 `json:"eventType"`
	EventTime    time.Time              `json:"eventTime"`
	DeviceID     string                 `json:"deviceId"`
	ChannelID    string                 `json:"channelId"`
	EventDetails map[string]interface{} `json:"eventDetails"`
}

// Result tracks the outcome of sending an event
type Result struct {
	Event      EventConfig `json:"event"`
	StatusCode int         `json:"statusCode"`
	Response   string      `json:"response"`
	Error      string      `json:"error,omitempty"`
	Duration   int64       `json:"durationMs"`
}

func init() {
	// Define command line flags
	flag.StringVar(&serverURL, "url", "http://localhost:8080/event", "API server URL")
	flag.StringVar(&username, "user", "", "Basic auth username")
	flag.StringVar(&password, "pass", "", "Basic auth password")
	flag.IntVar(&concurrency, "concurrency", 1, "Number of concurrent requests")
	flag.StringVar(&scenarioFile, "scenario", "test_scenario.json", "JSON file with test scenarios")
	flag.BoolVar(&outputResults, "output", false, "Output results to results.json")
}

func main() {
	flag.Parse()

	// Load test scenario
	scenario, err := loadScenario(scenarioFile)
	if err != nil {
		log.Fatalf("Failed to load scenario: %v", err)
	}

	fmt.Printf("Running scenario: %s\n", scenario.Name)
	fmt.Printf("Description: %s\n", scenario.Description)
	fmt.Printf("Events: %d\n", len(scenario.Events))
	fmt.Printf("Concurrency: %d\n", concurrency)
	fmt.Println("=======================")

	// Create a channel to hold the work and results
	jobs := make(chan EventConfig, len(scenario.Events))
	results := make(chan Result, len(scenario.Events))

	// Start worker pool
	var wg sync.WaitGroup
	for w := 1; w <= concurrency; w++ {
		wg.Add(1)
		go worker(w, jobs, results, &wg)
	}

	// Add jobs to the queue
	for _, event := range scenario.Events {
		jobs <- event
	}
	close(jobs) // Close the jobs channel when all jobs are added

	// Wait for all workers to finish in a separate goroutine
	go func() {
		wg.Wait()
		close(results) // Close results when all workers are done
	}()

	// Collect results
	var allResults []Result
	for result := range results {
		allResults = append(allResults, result)
		if result.Error != "" {
			fmt.Printf("❌ Error sending %s event: %s\n", result.Event.EventType, result.Error)
		} else {
			fmt.Printf("✅ Sent %s event to %s: Status %d (%dms)\n",
				result.Event.EventType,
				result.Event.DeviceID,
				result.StatusCode,
				result.Duration)
		}
	}

	// Output results if requested
	if outputResults && len(allResults) > 0 {
		resultsJSON, err := json.MarshalIndent(allResults, "", "  ")
		if err != nil {
			log.Printf("Failed to marshal results: %v", err)
		} else {
			err = os.WriteFile("results.json", resultsJSON, 0644)
			if err != nil {
				log.Printf("Failed to write results file: %v", err)
			} else {
				fmt.Println("Results written to results.json")
			}
		}
	}

	fmt.Println("=======================")
	fmt.Printf("Test scenario completed: %d events sent\n", len(allResults))
}

// worker processes jobs from the jobs channel
func worker(id int, jobs <-chan EventConfig, results chan<- Result, wg *sync.WaitGroup) {
	defer wg.Done()

	for j := range jobs {
		// Apply configured delay
		if j.DelaySeconds > 0 {
			time.Sleep(time.Duration(j.DelaySeconds) * time.Second)
		}

		// Send the event
		result := sendEvent(j)
		results <- result
	}
}

// sendEvent sends a single event to the API
func sendEvent(config EventConfig) Result {
	startTime := time.Now()
	result := Result{
		Event: config,
	}

	// Create the event
	event := VivotekEvent{
		EventType:    config.EventType,
		EventTime:    time.Now(),
		DeviceID:     config.DeviceID,
		ChannelID:    config.ChannelID,
		EventDetails: config.EventDetails,
	}

	// Marshal to JSON
	payload, err := json.Marshal(event)
	if err != nil {
		result.Error = fmt.Sprintf("error creating JSON payload: %v", err)
		return result
	}

	// Create the request
	req, err := http.NewRequest("POST", serverURL, bytes.NewBuffer(payload))
	if err != nil {
		result.Error = fmt.Sprintf("error creating request: %v", err)
		return result
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")

	// Add basic auth if credentials were provided
	if username != "" {
		req.SetBasicAuth(username, password)
	}

	// Send the request
	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		result.Error = fmt.Sprintf("error sending request: %v", err)
		return result
	}
	defer resp.Body.Close()

	// Record status code
	result.StatusCode = resp.StatusCode
	result.Duration = time.Since(startTime).Milliseconds()

	// Record error for non-2xx responses
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		result.Error = fmt.Sprintf("server returned status code %d", resp.StatusCode)
	}

	return result
}

// loadScenario loads a test scenario from a JSON file
func loadScenario(filename string) (*TestScenario, error) {
	file, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("error reading scenario file: %v", err)
	}

	var scenario TestScenario
	err = json.Unmarshal(file, &scenario)
	if err != nil {
		return nil, fmt.Errorf("error parsing scenario file: %v", err)
	}

	return &scenario, nil
}
