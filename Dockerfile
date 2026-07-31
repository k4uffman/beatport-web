FROM golang:1.22-alpine AS builder

# Install required libraries and git
RUN apk add --no-cache build-base taglib-dev cmake git

WORKDIR /app

# 1. Download and build the original beatportdl tool
RUN git clone https://github.com/unspok3n/beatportdl.git
WORKDIR /app/beatportdl
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/beatportdl-cli ./cmd/beatportdl

# 2. Build the web server we wrote
WORKDIR /app
COPY server.go .
RUN go mod init beatport-server
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./server.go

# 3. Create the final lightweight container
FROM alpine:latest

RUN apk add --no-cache taglib ca-certificates ffmpeg

WORKDIR /app
COPY --from=builder /app/beatportdl-cli .
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
