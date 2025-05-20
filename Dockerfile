FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY src/go.mod src/go.sum src/main.go ./
RUN go mod download

RUN go build -o /hello-device-plugin main.go

FROM alpine:3.19
WORKDIR /app
COPY --from=builder /hello-device-plugin ./hello-device-plugin

ENTRYPOINT ["/app/hello-device-plugin"]