package main

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
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
	HikEnabled      bool   `json:"hik_enabled"`
	HikUsername     string `json:"hik_username"`
	HikPassword     string `json:"hik_password"`
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

// HikVisionEvent represents the alarm data structure from HIKVision
type HikVisionEvent struct {
	EventType    string                 `json:"eventType"`
	EventTime    time.Time              `json:"eventTime"`
	DeviceID     string                 `json:"deviceId"`
	ChannelID    string                 `json:"channelId"`
	EventDetails map[string]interface{} `json:"eventDetails"`
	// Raw XML data for debugging/logging
	RawXML string `json:"-"`
}

// HIKVisionAlarm represents the XML structure of a HIKVision alarm event
type HIKVisionAlarm struct {
	XMLName          xml.Name `xml:"EventNotificationAlert"`
	IPAddress        string   `xml:"ipAddress"`
	PortNo           int      `xml:"portNo"`
	ProtocolType     string   `xml:"protocolType"`
	MacAddress       string   `xml:"macAddress"`
	ChannelID        int      `xml:"channelID"`
	DateTime         string   `xml:"dateTime"`
	ActivePostCount  int      `xml:"activePostCount"`
	EventType        string   `xml:"eventType"`
	EventState       string   `xml:"eventState"`
	EventDescription string   `xml:"eventDescription"`
	// Optional fields that may be present in some events
	DetectionRegionID int `xml:"detectionRegionID,omitempty"`
}

// GlobalState maintains the application state
type GlobalState struct {
	Config     Config
	EventCount int
	Logger     *log.Logger
}

var state GlobalState
var startTime time.Time

// initConfig loads configuration from a JSON file
func initConfig() error {
	// Default configuration
	state.Config = Config{
		ServerPort: "8080",
		LogFile:    "nvr_events.log",
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

	state.Logger = log.New(logOutput, "NVR-API: ", log.LstdFlags)
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
			w.Header().Set("WWW-Authenticate", `Basic realm="NVR API"`)
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

// handleHikVisionAlarm processes alarm events from HIKVision devices
func handleHikVisionAlarm(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodGet {
		w.WriteHeader(http.StatusMethodNotAllowed)
		w.Write([]byte("Only POST and GET methods are supported"))
		return
	}

	// Check for specific HIK authentication if enabled
	if state.Config.HikEnabled && state.Config.HikUsername != "" {
		username, password, ok := r.BasicAuth()
		if !ok || username != state.Config.HikUsername || password != state.Config.HikPassword {
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte("Unauthorized for HIKVision integration"))
			return
		}
	}

	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		state.Logger.Printf("Error reading HIKVision request body: %v", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Parse the XML alarm data
	var hikAlarm HIKVisionAlarm
	err = xml.Unmarshal(body, &hikAlarm)
	if err != nil {
		state.Logger.Printf("Error parsing HIKVision XML: %v", err)
		state.Logger.Printf("Raw payload: %s", string(body))
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	// Convert to our standard event format
	event := convertHikVisionAlarm(hikAlarm, string(body))

	// Log the event
	state.EventCount++
	state.Logger.Printf("Received HIKVision alarm #%d: Type=%s, Device=%s, Channel=%s",
		state.EventCount, event.EventType, event.DeviceID, event.ChannelID)

	// Process the event based on type
	processEvent(&event)

	// Respond with success
	w.WriteHeader(http.StatusOK)
	response := map[string]interface{}{
		"status":  "success",
		"message": "HIKVision alarm processed successfully",
		"eventId": state.EventCount,
	}

	// HIKVision may expect XML response, but most implementations work fine with JSON
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// convertHikVisionAlarm converts HIKVision alarm format to our standard event format
func convertHikVisionAlarm(hikAlarm HIKVisionAlarm, rawXML string) HikVisionEvent {
	// Parse the datetime from HIKVision format
	eventTime, err := time.Parse("2006-01-02T15:04:05-07:00", hikAlarm.DateTime)
	if err != nil {
		// If standard format fails, try alternative formats
		eventTime, err = time.Parse("2006-01-02T15:04:05Z", hikAlarm.DateTime)
		if err != nil {
			// If all parsing fails, use current time
			eventTime = time.Now()
		}
	}

	// Map HIKVision event types to standardized types
	eventType := mapHikEventType(hikAlarm.EventType)

	// Create device ID from IP if available
	deviceID := fmt.Sprintf("HIK_%s", hikAlarm.IPAddress)
	if hikAlarm.MacAddress != "" {
		deviceID = fmt.Sprintf("HIK_%s", strings.ReplaceAll(hikAlarm.MacAddress, ":", ""))
	}

	// Create channel ID
	channelID := fmt.Sprintf("Channel%d", hikAlarm.ChannelID)

	// Create event details map
	eventDetails := map[string]interface{}{
		"source":       "HIKVision",
		"ipAddress":    hikAlarm.IPAddress,
		"description":  hikAlarm.EventDescription,
		"state":        hikAlarm.EventState,
		"macAddress":   hikAlarm.MacAddress,
		"originalType": hikAlarm.EventType,
	}

	// Add optional fields if present
	if hikAlarm.DetectionRegionID > 0 {
		eventDetails["regionId"] = hikAlarm.DetectionRegionID
	}

	return HikVisionEvent{
		EventType:    eventType,
		EventTime:    eventTime,
		DeviceID:     deviceID,
		ChannelID:    channelID,
		EventDetails: eventDetails,
		RawXML:       rawXML,
	}
}

// mapHikEventType converts HIKVision event types to our standardized types
func mapHikEventType(hikType string) string {
	// Map HIKVision event types to standardized types
	// HIKVision has many event types, this is a simplified mapping
	hikType = strings.ToLower(hikType)

	switch {
	case strings.Contains(hikType, "motion"):
		return "MotionDetection"
	case strings.Contains(hikType, "videoloss"):
		return "VideoLoss"
	case strings.Contains(hikType, "tamper") || strings.Contains(hikType, "shelteralarm"):
		return "TamperDetection"
	case strings.Contains(hikType, "disk"):
		return "StorageFailure"
	case strings.Contains(hikType, "line") || strings.Contains(hikType, "crossing"):
		return "LineCrossing"
	case strings.Contains(hikType, "intrusion"):
		return "IntrusionDetection"
	case strings.Contains(hikType, "face"):
		return "FaceDetection"
	case strings.Contains(hikType, "io") || strings.Contains(hikType, "alarm"):
		return "IOAlarm"
	case strings.Contains(hikType, "connection"):
		return "DeviceConnection"
	default:
		return "UnknownEvent_" + hikType
	}
}

// processEvent handles different event types
func processEvent(event interface{}) {
	// Process based on event type
	switch e := event.(type) {
	case *VivotekEvent:
		switch e.EventType {
		case "MotionDetection":
			handleMotionEvent(e)
		case "VideoLoss":
			handleVideoLossEvent(e)
		case "DeviceConnection":
			handleConnectionEvent(e)
		default:
			state.Logger.Printf("Unhandled Vivotek event type: %s", e.EventType)
		}

		// Forward to notification URL if configured
		if state.Config.NotifyURL != "" {
			forwardEvent(e)
		}

	case *HikVisionEvent:
		switch e.EventType {
		case "MotionDetection":
			handleHikMotionEvent(e)
		case "VideoLoss":
			handleHikVideoLossEvent(e)
		case "LineCrossing", "IntrusionDetection":
			handleHikSmartEvent(e)
		case "IOAlarm":
			handleHikIOAlarmEvent(e)
		case "DeviceConnection":
			handleHikConnectionEvent(e)
		default:
			state.Logger.Printf("Unhandled HIKVision event type: %s", e.EventType)
		}

		// Forward to notification URL if configured
		if state.Config.NotifyURL != "" {
			forwardHikEvent(e)
		}
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

// handleHikMotionEvent processes HIKVision motion detection events
func handleHikMotionEvent(event *HikVisionEvent) {
	state.Logger.Printf("HIKVision motion detected on device %s, channel %s", event.DeviceID, event.ChannelID)
	// Add custom processing for HIKVision motion events
}

// handleHikVideoLossEvent processes HIKVision video loss events
func handleHikVideoLossEvent(event *HikVisionEvent) {
	state.Logger.Printf("HIKVision video lost on device %s, channel %s", event.DeviceID, event.ChannelID)
	// Add custom processing for HIKVision video loss events
}

// handleHikSmartEvent processes HIKVision smart events (line crossing, intrusion)
func handleHikSmartEvent(event *HikVisionEvent) {
	state.Logger.Printf("HIKVision smart event %s on device %s, channel %s",
		event.EventType, event.DeviceID, event.ChannelID)
	// Add custom processing for HIKVision smart events
}

// handleHikIOAlarmEvent processes HIKVision IO alarm events
func handleHikIOAlarmEvent(event *HikVisionEvent) {
	state.Logger.Printf("HIKVision IO alarm on device %s, channel %s", event.DeviceID, event.ChannelID)
	// Add custom processing for HIKVision IO events
}

// handleHikConnectionEvent processes HIKVision device connection events
func handleHikConnectionEvent(event *HikVisionEvent) {
	state.Logger.Printf("HIKVision connection event for device %s", event.DeviceID)
	// Add custom processing for HIKVision connection events
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

// forwardHikEvent forwards HIKVision events to notification URL
func forwardHikEvent(event *HikVisionEvent) {
	eventJSON, err := json.Marshal(event)
	if err != nil {
		state.Logger.Printf("Error serializing HIKVision event for forwarding: %v", err)
		return
	}

	resp, err := http.Post(state.Config.NotifyURL, "application/json", bytes.NewBuffer(eventJSON))
	if err != nil {
		state.Logger.Printf("Error forwarding HIKVision event: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		state.Logger.Printf("Error response from notification URL for HIKVision event: %d", resp.StatusCode)
	}
}

// sendTelegramNotification sends event information to a Telegram chat/bot
func sendTelegramNotification(event interface{}) {
	// Format the message based on event type
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
		// Log success based on event type
		switch e := event.(type) {
		case *VivotekEvent:
			state.Logger.Printf("Telegram notification sent successfully for Vivotek event type %s", e.EventType)
		case *HikVisionEvent:
			state.Logger.Printf("Telegram notification sent successfully for HIKVision event type %s", e.EventType)
		default:
			state.Logger.Printf("Telegram notification sent successfully for unknown event type")
		}
	}
}

// formatTelegramMessage creates a human-readable message for Telegram
func formatTelegramMessage(event interface{}) string {
	var message string

	switch e := event.(type) {
	case *VivotekEvent:
		// Basic message with event details
		message = fmt.Sprintf("<b>🚨 NVR Alert</b>\n\n"+
			"<b>Event:</b> %s\n"+
			"<b>Time:</b> %s\n"+
			"<b>Device:</b> %s\n"+
			"<b>Channel:</b> %s\n",
			e.EventType,
			e.EventTime.Format("2006-01-02 15:04:05"),
			e.DeviceID,
			e.ChannelID)

		// Add custom message based on event type
		switch e.EventType {
		case "MotionDetection":
			message += "📹 <b>Motion detected!</b>"

			// Add zone info if available
			if zone, ok := e.EventDetails["zoneId"].(string); ok {
				message += fmt.Sprintf(" (Zone: %s)", zone)
			}

		case "VideoLoss":
			message += "⚠️ <b>Video signal lost!</b> Please check camera connection."

		case "DeviceConnection":
			if status, ok := e.EventDetails["status"].(string); ok && status == "disconnected" {
				message += "❌ <b>Device disconnected!</b> Network issue possible."
			} else {
				message += "✅ <b>Device connected</b> and operating normally."
			}

		default:
			// Add any available details for unknown event types
			detailsJSON, _ := json.Marshal(e.EventDetails)
			if len(detailsJSON) > 0 {
				message += fmt.Sprintf("\n<pre>%s</pre>", string(detailsJSON))
			}
		}

	case *HikVisionEvent:
		// HIKVision specific formatting
		message = fmt.Sprintf("<b>🔔 HIKVision Alarm</b>\n\n"+
			"<b>Event:</b> %s\n"+
			"<b>Time:</b> %s\n"+
			"<b>Device:</b> %s\n"+
			"<b>Channel:</b> %s\n",
			e.EventType,
			e.EventTime.Format("2006-01-02 15:04:05"),
			e.DeviceID,
			e.ChannelID)

		// Add description if available
		if desc, ok := e.EventDetails["description"].(string); ok && desc != "" {
			message += fmt.Sprintf("<b>Description:</b> %s\n", desc)
		}

		// Add custom message based on event type
		switch e.EventType {
		case "MotionDetection":
			message += "📹 <b>Motion detected!</b>"

		case "LineCrossing":
			message += "🚷 <b>Line crossing detected!</b>"

		case "IntrusionDetection":
			message += "🚨 <b>Intrusion detected!</b>"

		case "FaceDetection":
			message += "👤 <b>Face detected!</b>"

		case "IOAlarm":
			message += "🔌 <b>I/O Alarm triggered!</b>"

		case "TamperDetection":
			message += "⚠️ <b>Camera tampering detected!</b>"

		case "VideoLoss":
			message += "⚠️ <b>Video signal lost!</b>"

		case "StorageFailure":
			message += "💾 <b>Storage failure!</b> Check NVR hard drive."

		default:
			// For unknown events, include available details
			if state, ok := e.EventDetails["state"].(string); ok {
				message += fmt.Sprintf("\n<b>State:</b> %s", state)
			}
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

	// Add HIKVision alarm server endpoint

	http.HandleFunc("/hikvision/alarm", basicAuth(handleHikVisionAlarm))

	// Start the HTTP server
	serverAddr := fmt.Sprintf(":%s", state.Config.ServerPort)
	state.Logger.Printf("Starting NVR Event Handler API on %s", serverAddr)
	fmt.Printf("Starting NVR Event Handler API on %s\n", serverAddr)
	if err := http.ListenAndServe(serverAddr, nil); err != nil {
		state.Logger.Fatalf("Failed to start server: %v", err)
	}
}
