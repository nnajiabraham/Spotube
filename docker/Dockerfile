# Build stage for Go backend
FROM golang:1.24-alpine AS builder-go

WORKDIR /app

# Copy Go module files
COPY backend/go.mod backend/go.sum ./
RUN go mod download

# Copy Go source code
COPY backend/ ./

# Build the Go binary
RUN CGO_ENABLED=0 GOOS=linux go build -o server cmd/server/main.go

# Build stage for React frontend
FROM node:20-alpine AS builder-node

WORKDIR /app

# Copy package files
COPY frontend/package*.json ./
RUN npm ci

# Copy frontend source
COPY frontend/ ./

# Build the frontend
RUN npm run build

# Runtime stage
FROM gcr.io/distroless/static:nonroot

WORKDIR /app

# Copy Go binary
COPY --from=builder-go /app/server ./server

# Copy frontend build artifacts  
COPY --from=builder-node /app/dist ./pb_public

# Expose port
EXPOSE 8090

# Run the server
CMD ["./server"] 