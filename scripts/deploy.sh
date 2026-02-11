#!/bin/bash

set -e

echo "🚀 Deploying new version..."

# Pull latest code
git pull origin main

# Build new image
echo "📦 Building..."
docker-compose build bot

# Update with zero-downtime
echo "🔄 Updating..."
docker-compose up -d --no-deps bot

# Wait for container to be healthy
echo "⏳ Waiting for health check..."
sleep 15

# Check health
if docker-compose exec -T bot wget -q --spider http://localhost:8080/health 2>/dev/null; then
    echo "✅ Deployment successful!"
else
    echo "⚠️ Health check failed, but container is running"
fi

# Show logs
echo "📋 Recent logs:"
docker-compose logs --tail=20 bot