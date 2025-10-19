#!/bin/bash

set -e

echo "Starting load balancer services..."

if ! docker info > /dev/null 2>&1; then
    echo "Error: Docker is not running"
    exit 1
fi

docker-compose up --build -d

echo ""
echo "Services started successfully!"
echo ""
echo "Available endpoints:"
echo "  Load Balancer: http://localhost:8080"
echo "  Prometheus:    http://localhost:9090"
echo "  Grafana:       http://localhost:3001 (admin/admin)"
echo ""
echo "Run 'docker-compose logs -f' to view logs"
