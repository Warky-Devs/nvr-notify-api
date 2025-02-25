package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Configuration for the application
type Config struct {
	ServerPort      string `json:"server_port"`
	LogFile         string `json:"log_file"`
	NotifyURL       string `json:"notify_url"`
	AuthUsername    string `json:"auth_username"`
	AuthPassword    string `json:"auth_password"`
	TelegramEnabled bool   `json:"telegram_enabled"`
	TelegramToken   string `json:"telegram_token"`
	TelegramChatID  string `json:"telegram_chat_id"`
}

// VivotekEvent represents the event data structure from Vivotek NVR
type VivotekEvent struct {
	EventType    string                 `json:"eventType"`
	EventTime    time.Time              `json:"eventTime"`
	DeviceID     string                 `json:"deviceId"`
	ChannelID    string                 `json:"channelId"`
	EventDetails map[string]interface{} `json:"eventDetails"`
	// Add more fields as needed based on Vivotek's event structure
}

// GlobalState maintains the application state
type GlobalState struct {
	Config     Config
	EventCount int
	Logger     *log.Logger
}

var state GlobalState

// initConfig loads configuration from a JSON file
func initConfig() error {
	// Default configuration
	state.Config = Config{
		ServerPort: "8080",
		LogFile:    "vivotek_events.log",
	}

	// Try to load from config file if it exists
	configFile, err := os.Open("config.json")
	if err == nil {
		defer configFile.Close()
		decoder := json.NewDecoder(configFile)
		err = decoder.Decode(&state.Config)
		if err != nil {
			return fmt.Errorf("error parsing config file: %v", err)
		}
	}

	// Initialize logger
	var logOutput io.Writer
	if state.Config.LogFile == "stdout" {
		logOutput = os.Stdout
	} else {
		file, err := os.OpenFile(state.Config.LogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
		if err != nil {
			return fmt.Errorf("failed to open log file: %v", err)
		}
		logOutput = file
	}

	state.Logger = log.New(logOutput, "VIVOTEK-API: ", log.LstdFlags)

	fmt.Printf("Config loaded, handing off logs to %s...\n", state.Config.LogFile)
	return nil
}

// basicAuth implements HTTP Basic Authentication middleware
func basicAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		// Skip auth if credentials are not configured
		if state.Config.AuthUsername == "" || state.Config.AuthPassword == "" {
			next(w, r)
			return
		}

		username, password, ok := r.BasicAuth()
		if !ok || username != state.Config.AuthUsername || password != state.Config.AuthPassword {
			w.Header().Set("WWW-Authenticate", `Basic realm="Vivotek NVR API"`)
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized"))
			return
		}

		next(w, r)
	}
}

// handleEvent processes events from Vivotek NVR
func handleEvent(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Only POST method is supported"))
		return
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		state.Logger.Printf("Error reading request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse the event
	var event VivotekEvent
	if err := json.Unmarshal(body, &event); err != nil {
		state.Logger.Printf("Error parsing event JSON: %v", err)
		state.Logger.Printf("Raw payload: %s", string(body))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Log the event
	state.EventCount++
	state.Logger.Printf("Received event #%d: Type=%s, Device=%s, Channel=%s",
		state.EventCount, event.EventType, event.DeviceID, event.ChannelID)

	// Process the event based on type
	processEvent(&event)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"status":  "success",
		"message": "Event processed successfully",
		"eventId": state.EventCount,
	}

	json.NewEncoder(w).Encode(response)
}

// processEvent handles different event types
func processEvent(event *VivotekEvent) {
	switch event.EventType {
	case "MotionDetection":
		handleMotionEvent(event)
	case "VideoLoss":
		handleVideoLossEvent(event)
	case "DeviceConnection":
		handleConnectionEvent(event)
	default:
		state.Logger.Printf("Unhandled event type: %s", event.EventType)
	}

	// Forward to notification URL if configured
	if state.Config.NotifyURL != "" {
		forwardEvent(event)
	}

	// Send to Telegram if enabled
	if state.Config.TelegramEnabled && state.Config.TelegramToken != "" && state.Config.TelegramChatID != "" {
		sendTelegramNotification(event)
	}
}

// handleMotionEvent processes motion detection events
func handleMotionEvent(event *VivotekEvent) {
	state.Logger.Printf("Motion detected on device %s, channel %s", event.DeviceID, event.ChannelID)
	// Add custom processing for motion events
}

// handleVideoLossEvent processes video loss events
func handleVideoLossEvent(event *VivotekEvent) {
	state.Logger.Printf("Video lost on device %s, channel %s", event.DeviceID, event.ChannelID)
	// Add custom processing for video loss events
}

// handleConnectionEvent processes device connection/disconnection events
func handleConnectionEvent(event *VivotekEvent) {
	state.Logger.Printf("Connection event for device %s", event.DeviceID)
	// Add custom processing for connection events
}

// forwardEvent sends the event to a configured notification URL
func forwardEvent(event *VivotekEvent) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		state.Logger.Printf("Error serializing event for forwarding: %v", err)
		return
	}

	resp, err := http.Post(state.Config.NotifyURL, "application/json", bytes.NewBuffer(eventJSON))
	if err != nil {
		state.Logger.Printf("Error forwarding event: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		state.Logger.Printf("Error response from notification URL: %d", resp.StatusCode)
	}
}

// sendTelegramNotification sends event information to a Telegram chat/bot
func sendTelegramNotification(event *VivotekEvent) {
	// Format the message
	message := formatTelegramMessage(event)

	// Construct the Telegram Bot API URL
	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", state.Config.TelegramToken)

	// Prepare the request data
	data := url.Values{}
	data.Set("chat_id", state.Config.TelegramChatID)
	data.Set("text", message)
	data.Set("parse_mode", "HTML") // Enable HTML formatting

	// Send the request
	resp, err := http.PostForm(apiURL, data)
	if err != nil {
		state.Logger.Printf("Error sending Telegram notification: %v", err)
		return
	}
	defer resp.Body.Close()

	// Check for error response
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		state.Logger.Printf("Telegram API error: status=%d, response=%s", resp.StatusCode, string(body))
	} else {
		state.Logger.Printf("Telegram notification sent successfully for event type %s", event.EventType)
	}
}

// formatTelegramMessage creates a human-readable message for Telegram
func formatTelegramMessage(event *VivotekEvent) string {
	// Basic message with event details
	message := fmt.Sprintf("<b>🚨 Vivotek NVR Alert</b>\n\n"+
		"<b>Event:</b> %s\n"+
		"<b>Time:</b> %s\n"+
		"<b>Device:</b> %s\n"+
		"<b>Channel:</b> %s\n",
		event.EventType,
		event.EventTime.Format("2006-01-02 15:04:05"),
		event.DeviceID,
		event.ChannelID)

	// Add custom message based on event type
	switch event.EventType {
	case "MotionDetection":
		message += "📹 <b>Motion detected!</b>"

		// Add zone info if available
		if zone, ok := event.EventDetails["zoneId"].(string); ok {
			message += fmt.Sprintf(" (Zone: %s)", zone)
		}

	case "VideoLoss":
		message += "⚠️ <b>Video signal lost!</b> Please check camera connection."

	case "DeviceConnection":
		if status, ok := event.EventDetails["status"].(string); ok && status == "disconnected" {
			message += "❌ <b>Device disconnected!</b> Network issue possible."
		} else {
			message += "✅ <b>Device connected</b> and operating normally."
		}

	default:
		// Add any available details for unknown event types
		detailsJSON, _ := json.Marshal(event.EventDetails)
		if len(detailsJSON) > 0 {
			message += fmt.Sprintf("\n<pre>%s</pre>", string(detailsJSON))
		}
	}

	return message
}

// healthCheck provides a simple endpoint to verify the service is running
func healthCheck(w http.ResponseWriter, r *http.Request) {
	response := map[string]interface{}{
		"status":     "ok",
		"eventCount": state.EventCount,
		"uptime":     time.Since(startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

var startTime time.Time

func main() {
	startTime = time.Now()
	fmt.Print("Starting NVR API...\n")

	// Initialize configuration
	if err := initConfig(); err != nil {
		log.Fatalf("Failed to initialize configuration: %v", err)
	}

	// Set up HTTP routes
	http.HandleFunc("/health", healthCheck)
	http.HandleFunc("/event", basicAuth(handleEvent))
	http.HandleFunc("/events", basicAuth(handleEvent)) // Alternative endpoint

	// Start the HTTP server
	serverAddr := fmt.Sprintf(":%s", state.Config.ServerPort)
	state.Logger.Printf("Starting NVR Event Handler API on %s", serverAddr)
	fmt.Printf("Starting NVR Event Handler API on %s\n", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		state.Logger.Fatalf("Failed to start server: %v", err)
	}
}
