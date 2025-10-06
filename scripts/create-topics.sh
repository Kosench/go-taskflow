#!/bin/bash

# Color output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

KAFKA_CONTAINER="task-queue-kafka"
KAFKA_BOOTSTRAP_SERVER="localhost:9092"

echo -e "${GREEN}Creating Kafka topics...${NC}"

# Wait for Kafka to be ready
echo -e "${YELLOW}Waiting for Kafka to be ready...${NC}"
sleep 10

# Function to create topic
create_topic() {
    local topic_name=$1
    local partitions=$2
    local replication_factor=$3
    local configs=$4

    echo -e "${GREEN}Creating topic: ${topic_name}${NC}"

    docker exec ${KAFKA_CONTAINER} kafka-topics \
        --create \
        --bootstrap-server ${KAFKA_BOOTSTRAP_SERVER} \
        --topic ${topic_name} \
        --partitions ${partitions} \
        --replication-factor ${replication_factor} \
        --config ${configs} \
        --if-not-exists

    if [ $? -eq 0 ]; then
        echo -e "${GREEN}✓ Topic '${topic_name}' created successfully${NC}"
    else
        echo -e "${RED}✗ Failed to create topic '${topic_name}'${NC}"
    fi
}

# Create topics
create_topic "tasks-high" 3 1 "retention.ms=604800000,compression.type=snappy"
create_topic "tasks-normal" 3 1 "retention.ms=604800000,compression.type=snappy"
create_topic "tasks-low" 2 1 "retention.ms=604800000,compression.type=snappy"
create_topic "tasks-retry" 2 1 "retention.ms=604800000,compression.type=snappy"
create_topic "tasks-dlq" 1 1 "retention.ms=2592000000,compression.type=gzip"
create_topic "task-results" 3 1 "retention.ms=604800000,compression.type=snappy,cleanup.policy=compact"

echo -e "${GREEN}Listing all topics:${NC}"
docker exec ${KAFKA_CONTAINER} kafka-topics \
    --list \
    --bootstrap-server ${KAFKA_BOOTSTRAP_SERVER}

echo -e "${GREEN}✓ All topics created!${NC}"