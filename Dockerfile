FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY . .
RUN go mod download
RUN CGO_ENABLED=0 go build -o loadbalancer ./cmd/server

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/loadbalancer .
EXPOSE 8080
CMD ["./loadbalancer"]
