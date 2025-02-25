# nvr-notify-api


A Go-based API server that receives and processes HTTP event notifications from Network Video Recorders (NVRs) and IP cameras.

## Features

- Supports multiple NVR brands:
  - Vivotek NVR JSON events
  - HIKVision alarm server XML notifications
- Processes common event types:
  - Motion detection
  - Video loss
  - Device connection/disconnection
  - Line crossing (HIKVision)
  - Intrusion detection (HIKVision)
  - IO alarms (HIKVision)
- Configurable logging system
- Optional HTTP Basic Authentication
- Event forwarding to external services
- Telegram integration for instant notifications
- Health check endpoint
- Docker support

## Configuration

The application can be configured using the `config.json` file:

```json
{
  "server_port": "8080",
  "log_file": "nvr_events.log",
  "notify_url": "https://your-notification-service.com/webhook",
  "auth_username": "admin",
  "auth_password": "your-secure-password",
  "telegram_enabled": true,
  "telegram_token": "YOUR_TELEGRAM_BOT_TOKEN",
  "telegram_chat_id": "YOUR_CHAT_ID",
  "hik_enabled": true,
  "hik_username": "hikvision",
  "hik_password": "hikvision-password"
}
```

Configuration options:
- `server_port`: Port the HTTP server will listen on
- `log_file`: Path to log file (use "stdout" to log to console)
- `notify_url`: Optional URL to forward events to
- `auth_username` and `auth_password`: Basic Authentication credentials
- `telegram_enabled`: Set to true to enable Telegram notifications
- `telegram_token`: Your Telegram bot token (obtained from @BotFather)
- `telegram_chat_id`: Your Telegram chat ID where notifications should be sent
- `hik_enabled`: Set to true to enable HIKVision-specific authentication
- `hik_username` and `hik_password`: Optional HIKVision-specific auth credentials

## API Endpoints

- `/event` or `/events`: POST endpoint for receiving Vivotek NVR event notifications
- `/hikvision/alarm`: POST endpoint for receiving HIKVision alarm server notifications
- `/health`: GET endpoint to check service status

## Event Format

### Vivotek Events
The API expects Vivotek events in JSON format:

```json
{
  "eventType": "MotionDetection",
  "eventTime": "2023-06-15T14:30:00Z",
  "deviceId": "NVR123456",
  "channelId": "Camera01",
  "eventDetails": {
    "zoneId": "Zone1",
    "confidence": 85
  }
}
```

### HIKVision Events
HIKVision events are expected in XML format according to the HIKVision alarm server protocol:

```xml
<EventNotificationAlert>
  <ipAddress>192.168.1.64</ipAddress>
  <portNo>80</portNo>
  <protocolType>HTTP</protocolType>
  <macAddress>00:11:22:33:44:55</macAddress>
  <channelID>1</channelID>