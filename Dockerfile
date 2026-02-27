#This file is not verified. 

# --- Stage 1: The Builder ---
FROM golang:1.25 AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go modules manifests first (for caching)
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the source code
COPY . .

# Build the Go application. 
# We disable CGO to ensure it's a statically linked binary.
RUN CGO_ENABLED=0 GOOS=linux go build -o workflow-engine ./cmd/api/main.go

# --- Stage 2: The Final Minimal Image ---
FROM alpine:latest  

WORKDIR /root/

# Copy the compiled binary from the builder stage
COPY --from=builder /app/workflow-engine .

# Expose the port your Gin API runs on
EXPOSE 8080

# Command to run the executable
CMD ["./workflow-engine"]