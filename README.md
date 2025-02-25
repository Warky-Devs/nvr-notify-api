# nvr-notify-api

A Go-based API server that receives and processes HTTP event notifications from Vivotek Network Video Recorders (NVRs).

## Features

- Receives event notifications from Vivotek NVR devices
- Processes different event types (motion detection, video loss, device connection)
- Configurable logging
- Optional HTTP Basic Authentication
- Event forwarding to external notification services
- Telegram integration for instant notifications
- Health check endpoint
- Docker support

## Configuration

The application can be configured using the `config.json` file:

```json
{
  "server_port": "8080",
  "log_file": "vivotek_events.log",
  "notify_url": "https://your-notification-service.com/webhook",
  "auth_username": "admin",
  "auth_password": "your-secure-password",
  "telegram_enabled": true,
  "telegram_token": "YOUR_TELEGRAM_BOT_TOKEN",
  "telegram_chat_id": "YOUR_CHAT_ID"
}
```

Configuration options:
- `server_port`: Port the HTTP server will listen on
- `log_file`: Path to log file (use "stdout" to log to console)
- `notify_url`: Optional URL to forward events to
- `auth_username` and `auth_password`: Optional Basic Authentication credentials
- `telegram_enabled`: Set to true to enable Telegram notifications
- `telegram_token`: Your Telegram bot token (obtained from @BotFather)
- `telegram_chat_id`: Your Telegram chat ID where notifications should be sent

## API Endpoints

- `/event` or `/events`: POST endpoint for receiving event notifications
- `/health`: GET endpoint to check service status

## Event Format

The API expects events in JSON format:

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

## Running the Application

### Directly

```bash
go mod tidy
go build
./vivotek-nvr-api
```

### Using Docker

```bash
# Build the Docker image
docker build -t vivotek-nvr-api .

# Run the container
docker run -p 8080:8080 -v ./config.json:/app/config.json -v ./logs:/app/logs vivotek-nvr-api
```

## Configuring Vivotek NVR

To configure your Vivotek NVR to send events to this API:

1. Access your NVR's web interface
2. Navigate to Configuration > Event > HTTP Notification
3. Enable HTTP notifications
4. Set the URL to `http://your-server-ip:8080/event`
5. Set the authentication method if you've configured it in the API
6. Select the events you want to be notified about
7. Save the configuration

## Extending the API

To handle additional event types, modify the `processEvent` function in the main Go file and add appropriate handler functions.

## Setting Up Telegram Notifications

1. Create a Telegram bot:
   - Start a chat with [@BotFather](https://t.me/botfather) on Telegram
   - Send the command `/newbot` and follow the instructions
   - Once created, BotFather will provide a token - copy this to your config file

2. Get your chat ID:
   - Option 1: Start a chat with your bot and send a message to it
   - Option 2: Send a message to [@userinfobot](https://t.me/userinfobot) to get your chat ID
   - Option 3: For group chats, add [@RawDataBot](https://t.me/RawDataBot) to your group briefly

3. Update your config file:
   - Set `telegram_enabled` to `true`
   - Add your bot token to `telegram_token`
   - Add your chat ID to `telegram_chat_id`

4. Test the configuration:
   - Start the API server
   - Trigger an event from your Vivotek NVR
   - You should receive a formatted message in your Telegram chat