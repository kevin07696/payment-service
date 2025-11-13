#!/bin/sh
set -e

echo "🚀 Starting payment-server entrypoint..."

# Database connection details from environment
DB_HOST="${DB_HOST:-postgres}"
DB_PORT="${DB_PORT:-5432}"
DB_USER="${DB_USER:-postgres}"
DB_PASSWORD="${DB_PASSWORD:-postgres}"
DB_NAME="${DB_NAME:-payment_service}"
DB_SSL_MODE="${DB_SSL_MODE:-disable}"

# Construct connection string for goose
DB_URL="postgres://${DB_USER}:${DB_PASSWORD}@${DB_HOST}:${DB_PORT}/${DB_NAME}?sslmode=${DB_SSL_MODE}"

echo "📊 Database: ${DB_HOST}:${DB_PORT}/${DB_NAME}"

# Wait for database to be ready
echo "⏳ Waiting for database to be ready..."
until pg_isready -h "${DB_HOST}" -p "${DB_PORT}" -U "${DB_USER}" >/dev/null 2>&1; do
    echo "   Database not ready, waiting..."
    sleep 2
done
echo "✅ Database is ready!"

# Run migrations
echo "🔄 Running database migrations..."
cd /home/appuser/migrations

if goose postgres "${DB_URL}" up; then
    echo "✅ Migrations completed successfully!"
else
    echo "❌ Migration failed!"
    exit 1
fi

# Show migration status
echo "📋 Current migration status:"
goose postgres "${DB_URL}" status

echo ""
echo "🎯 Starting payment-server..."
cd /home/appuser

# Execute the main application
exec ./payment-server
