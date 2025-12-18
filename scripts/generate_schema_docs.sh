#!/bin/bash
# generate_schema_docs.sh - Generates database schema documentation from migrations
# Usage: ./scripts/generate_schema_docs.sh

set -e

echo "📊 Generating database schema documentation..."

OUTPUT_FILE="docs/development/DATABASE.md"
MIGRATIONS_DIR="internal/db/migrations"

# Create output file
cat > "$OUTPUT_FILE" <<EOF
# Database Schema Reference

**Auto-Generated:** $(date '+%Y-%m-%d %H:%M:%S')
**Source:** Goose migrations in \`$MIGRATIONS_DIR\`

---

## Overview

This document is auto-generated from database migration files. Each migration represents a change to the database schema over time.

**Migration Tool:** Goose
**Migration Directory:** \`$MIGRATIONS_DIR\`

**How to read this:**
- Migrations are listed chronologically
- Each migration shows the SQL executed
- Use \`make migrate-status\` to see which migrations are applied

---

## Migrations

EOF

# Process each migration file
for migration in "$MIGRATIONS_DIR"/*.sql; do
    # Skip if no migrations found
    [ -f "$migration" ] || continue

    filename=$(basename "$migration")

    # Extract timestamp and name from filename (format: YYYYMMDDHHMMSS_name.sql)
    if [[ "$filename" =~ ^([0-9]{14})_(.+)\.sql$ ]]; then
        timestamp="${BASH_REMATCH[1]}"
        name="${BASH_REMATCH[2]}"

        # Format timestamp as readable date
        year=${timestamp:0:4}
        month=${timestamp:4:2}
        day=${timestamp:6:2}
        hour=${timestamp:8:2}
        minute=${timestamp:10:2}
        sec=${timestamp:12:2}
        readable_date="$year-$month-$day $hour:$minute:$sec"

        echo "### $name" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo "**Migration:** \`$filename\`  " >> "$OUTPUT_FILE"
        echo "**Created:** $readable_date" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo '```sql' >> "$OUTPUT_FILE"
        cat "$migration" >> "$OUTPUT_FILE"
        echo '```' >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo "---" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
    else
        # Fallback for files that don't match pattern
        echo "### $filename" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo '```sql' >> "$OUTPUT_FILE"
        cat "$migration" >> "$OUTPUT_FILE"
        echo '```' >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
        echo "---" >> "$OUTPUT_FILE"
        echo "" >> "$OUTPUT_FILE"
    fi
done

# Add footer
cat >> "$OUTPUT_FILE" <<EOF

## How to Apply Migrations

\`\`\`bash
# Check current migration status
make migrate-status

# Apply all pending migrations
make migrate-up

# Rollback last migration
make migrate-down

# Create new migration
make migrate-create NAME=add_new_table
\`\`\`

## Related Documentation

- **[DATABASE.md](./DATABASE.md)** - Database architecture and design decisions
- **[SETUP.md](./SETUP.md)** - Database setup for development
EOF

echo "✅ Schema documentation generated: $OUTPUT_FILE"
echo "   Found $(ls -1 $MIGRATIONS_DIR/*.sql 2>/dev/null | wc -l) migration files"
