#!/bin/bash

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

show_help() {
    echo -e "${BLUE}Task Queue Infrastructure Management${NC}"
    echo ""
    echo "Usage: ./scripts/infra.sh [command]"
    echo ""
    echo "Commands:"
    echo "  up        Start all infrastructure services"
    echo "  down      Stop all infrastructure services"
    echo "  restart   Restart all infrastructure services"
    echo "  logs      Show logs from all services"
    echo "  status    Show status of all services"
    echo "  clean     Stop services and remove volumes (WARNING: destroys data)"
    echo "  topics    Create Kafka topics"
    echo ""
}

start_infrastructure() {
    echo -e "${GREEN}Starting infrastructure...${NC}"
    docker-compose up -d

    echo -e "${YELLOW}Waiting for services to be healthy...${NC}"
    sleep 10

    # Check if services are running
    if docker-compose ps | grep -q "Up"; then
        echo -e "${GREEN}✓ Infrastructure is up and running!${NC}"
        echo ""
        echo -e "${BLUE}Services available at:${NC}"
        echo -e "  PostgreSQL:    localhost:5432"
        echo -e "  Kafka:         localhost:9092"
        echo -e "  Kafka UI:      http://localhost:8080"
        echo -e "  Prometheus:    http://localhost:9090"
        echo -e "  Grafana:       http://localhost:3000 (admin/admin)"
        echo ""

        # Create Kafka topics
        echo -e "${YELLOW}Creating Kafka topics...${NC}"
        ./scripts/create-topics.sh
    else
        echo -e "${RED}✗ Failed to start some services${NC}"
        docker-compose ps
        exit 1
    fi
}

stop_infrastructure() {
    echo -e "${YELLOW}Stopping infrastructure...${NC}"
    docker-compose down
    echo -e "${GREEN}✓ Infrastructure stopped${NC}"
}

restart_infrastructure() {
    stop_infrastructure
    start_infrastructure
}

show_logs() {
    docker-compose logs -f
}

show_status() {
    echo -e "${BLUE}Infrastructure Status:${NC}"
    docker-compose ps
}

clean_infrastructure() {
    echo -e "${RED}WARNING: This will delete all data!${NC}"
    read -p "Are you sure? (y/N): " -n 1 -r
    echo
    if [[ $REPLY =~ ^[Yy]$ ]]; then
        echo -e "${YELLOW}Cleaning infrastructure...${NC}"
        docker-compose down -v
        echo -e "${GREEN}✓ Infrastructure cleaned${NC}"
    else
        echo -e "${YELLOW}Cancelled${NC}"
    fi
}

create_topics() {
    ./scripts/create-topics.sh
}

# Main script
case "$1" in
    up)
        start_infrastructure
        ;;
    down)
        stop_infrastructure
        ;;
    restart)
        restart_infrastructure
        ;;
    logs)
        show_logs
        ;;
    status)
        show_status
        ;;
    clean)
        clean_infrastructure
        ;;
    topics)
        create_topics
        ;;
    *)
        show_help
        ;;
esac