# 1. Use the bleeding-edge version of Alpine
FROM alpine:edge AS builder

# Install Go, Git, C++ build tools, AND zlib-dev (The missing piece!)
RUN apk add --no-cache go build-base taglib-dev cmake git zlib-dev

WORKDIR /app

# 2. Download and build the original beatportdl tool
RUN git clone https://github.com/unspok3n/beatportdl.git
WORKDIR /app/beatportdl
RUN CGO_ENABLED=1 GOOS=linux go build -o /app/beatportdl-cli ./cmd/beatportdl

# 3. Build the web server we wrote
WORKDIR /app
COPY server.go .
RUN go mod init beatport-server
RUN go mod tidy
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./server.go

# 4. Create the final lightweight container
FROM alpine:edge

# Install the runtime dependencies
RUN apk add --no-cache taglib ca-certificates ffmpeg zlib

WORKDIR /app
COPY --from=builder /app/beatportdl-cli .
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
