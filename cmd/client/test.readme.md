# Vivotek NVR API Test Client

This repository contains two test clients for the Vivotek NVR Event Handler API:

1. **Single Event Test Client** (`vivotek-test-client.go`) - For sending individual test events with customizable parameters
2. **Batch Test Client** (`vivotek-test-batch.go`) - For running complex test scenarios defined in JSON

## Prerequisites

- Go 1.18 or higher
- Network access to your Vivotek NVR API server

## Single Event Test Client

### Building

```bash
go build -o vivotek-test vivotek-test-client.go
```

### Usage

```bash
./vivotek-test [options]
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-url` | API server URL | `http://localhost:8080/event` |
| `-type` | Event type | `MotionDetection` |
| `-device` | Device ID | `NVR12345` |
| `-channel` | Channel ID | `Camera01` |
| `-zone` | Detection zone (for motion events) | `Zone1` |
| `-user` | Basic auth username | `""` |
| `-pass` | Basic auth password | `""` |
| `-insecure` | Skip TLS verification | `false` |
| `-repeat` | Number of events to send | `1` |
| `-interval` | Interval between events in seconds | `5` |

### Examples

Test a motion detection event:
```bash
./vivotek-test -type=MotionDetection -device=NVR001 -channel=Camera01 -zone=FrontDoor
```

Test video loss with authentication:
```bash
./vivotek-test -type=VideoLoss -device=NVR001 -channel=Camera02 -user=admin -pass=password
```

Generate multiple events:
```bash
./vivotek-test -type=DeviceConnection -device=NVR002 -repeat=5 -interval=10
```

## Batch Test Client

The batch test client allows you to define complex test scenarios in a JSON file and execute them with a single command.

### Building

```bash
go build -o vivotek-batch vivotek-test-batch.go
```

### Usage

```bash
./vivotek-batch [options]
```

### Options

| Flag | Description | Default |
|------|-------------|---------|
| `-url` | API server URL | `http://localhost:8080/event` |
| `-user` | Basic auth username | `""` |
| `-pass` | Basic auth password | `""` |
| `-concurrency` | Number of concurrent requests | `1` |
| `-scenario` | JSON file with test scenarios | `test_scenario.json` |
| `-output` | Output results to results.json | `false` |

### Test Scenario Format

The test scenario file uses the following format:

```json
{
  "name": "Test Scenario Name",
  "description": "Description of the test scenario",
  "events": [
    {
      "eventType": "MotionDetection",
      "deviceId": "NVR001",
      "channelId": "Camera01",
      "delaySeconds": 0,
      "eventDetails": {
        "zoneId": "Zone1",
        "confidence": 95
      }
    },
    ...
  ]
}
```

### Examples

Run the default test scenario:
```bash
./vivotek-batch
```

Run a custom scenario with authentication:
```bash
./vivotek-batch -scenario=my_scenario.json -user=admin -pass=password
```

Run with multiple concurrent requests:
```bash
./vivotek-batch -concurrency=5 -output
```

## Tips for Testing

1. **Start with Single Events**: Use the single event client first to verify basic connectivity
2. **Check Server Logs**: Monitor the server logs while running tests to see how events are being processed
3. **Verify Telegram**: If you've configured Telegram notifications, check that they're being sent
4. **Increase Load Gradually**: When testing performance, start with low concurrency and gradually increase
5. **Custom Scenarios**: Create different scenarios for different testing purposes (basic functionality, stress testing, etc.)