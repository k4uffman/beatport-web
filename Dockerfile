FROM golang:1.22-alpine AS builder

RUN apk add --no-cache build-base taglib-dev cmake

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download || true

COPY . .

RUN CGO_ENABLED=1 GOOS=linux go build -o beatportdl-cli ./cmd/beatportdl || true
RUN CGO_ENABLED=1 GOOS=linux go build -o server ./server.go

FROM alpine:latest

RUN apk add --no-cache taglib ca-certificates ffmpeg

WORKDIR /app
COPY --from=builder /app/beatportdl-cli .
COPY --from=builder /app/server .

EXPOSE 8080
CMD ["./server"]
