#!/bin/bash

GREEN='\033[0;32m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${GREEN}Checking database connection...${NC}"

# Check if PostgreSQL is running
if docker exec task-queue-postgres pg_isready -U taskqueue > /dev/null 2>&1; then
    echo -e "${GREEN}✓ PostgreSQL is running${NC}"
else
    echo -e "${RED}✗ PostgreSQL is not running${NC}"
    exit 1
fi

# Check if database exists
if docker exec task-queue-postgres psql -U taskqueue -d taskqueue -c "SELECT 1" > /dev/null 2>&1; then
    echo -e "${GREEN}✓ Database 'taskqueue' exists${NC}"
else
    echo -e "${RED}✗ Database 'taskqueue' does not exist${NC}"
    exit 1
fi

# Show table count
TABLE_COUNT=$(docker exec task-queue-postgres psql -U taskqueue -d taskqueue -t -c "SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = 'public'")
echo -e "${GREEN}Tables in database: ${TABLE_COUNT}${NC}"

# Show tasks table structure if it exists
if docker exec task-queue-postgres psql -U taskqueue -d taskqueue -c "\d tasks" > /dev/null 2>&1; then
    echo -e "${GREEN}Tasks table structure:${NC}"
    docker exec task-queue-postgres psql -U taskqueue -d taskqueue -c "\d tasks"
fi

echo -e "${GREEN}Database check completed!${NC}"