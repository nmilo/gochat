# Stage 1: Build the Go application
FROM golang:1.23.1-alpine AS builder

# Set the Current Working Directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies. Dependencies will be cached if the go.mod and go.sum files are not changed
RUN go mod download

# Copy the source code into the container
COPY . .

# Build the Go application
RUN go build -o gochat .

# Stage 2: Run the Go application
FROM alpine:3.20.3

# Install ncurses for TUI
RUN apk add --no-cache ncurses ncurses-dev

# Set the working directory
WORKDIR /root/

# Copy the binary from builder
COPY --from=builder /app/gochat .

# Define an environment variable for the port
ENV PORT 4545

# Expose the port (will be changed dynamically)
EXPOSE ${PORT}

# Run the Go app
CMD ["./gochat"]
