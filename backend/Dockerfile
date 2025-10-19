FROM golang:1.24-alpine AS builder

WORKDIR /app
COPY main.go .
RUN go mod init backend && \
    CGO_ENABLED=0 go build -o backend main.go

FROM alpine:latest
WORKDIR /app
COPY --from=builder /app/backend .
CMD ["./backend"]
