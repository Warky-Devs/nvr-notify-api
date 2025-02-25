FROM golang:1.22-alpine AS builder
EXPOSE 8080

WORKDIR /app

# Copy go.mod and go.sum files
COPY go.mod ./
COPY go.sum ./

# Download dependencies
RUN go mod download

# Copy source code
#COPY *.go ./

# Copy source code
COPY ./cmd ./cmd

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o nvr-api cmd/apisrv/main.go

# Create a minimal production image
FROM alpine:latest

WORKDIR /app

# Copy the executable from the builder stage
COPY --from=builder /app/nvr-api .

# Copy config file
COPY config.sample.json /app/config.json

# Create directory for logs
RUN mkdir -p /app/logs


# Run the application
CMD ["./nvr-api"]