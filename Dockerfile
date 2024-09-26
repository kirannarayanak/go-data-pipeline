# Use the official Golang image
FROM golang:1.18-alpine

# Set the working directory
WORKDIR /app

# Copy the Go module files and download dependencies
COPY go.mod go.sum ./
RUN go mod download

# Copy the rest of the application code
COPY . .

# Build the application
RUN go build -o google-mercent-pipeline

# Command to run the binary
CMD ["./google-mercent-pipeline"]
