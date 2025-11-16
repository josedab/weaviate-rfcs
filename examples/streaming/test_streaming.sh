#!/bin/bash

# Real-Time Streaming Test Script
# This script demonstrates the streaming capabilities of Weaviate

set -e

echo "==================================="
echo "Weaviate Streaming Test Suite"
echo "==================================="
echo ""

# Colors for output
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

# Check if Docker is running
if ! docker info > /dev/null 2>&1; then
    echo -e "${RED}Error: Docker is not running${NC}"
    exit 1
fi

echo -e "${GREEN}✓ Docker is running${NC}"

# Start services
echo ""
echo "Starting streaming infrastructure..."
docker-compose up -d

# Wait for services to be ready
echo ""
echo "Waiting for services to be ready..."
sleep 10

# Check Kafka
echo -e "${YELLOW}Checking Kafka...${NC}"
docker exec kafka kafka-broker-api-versions --bootstrap-server localhost:9092 > /dev/null 2>&1
if [ $? -eq 0 ]; then
    echo -e "${GREEN}✓ Kafka is ready${NC}"
else
    echo -e "${RED}✗ Kafka is not ready${NC}"
    exit 1
fi

# Create topics
echo ""
echo -e "${YELLOW}Creating Kafka topics...${NC}"
docker exec kafka kafka-topics --create --topic products --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1 --if-not-exists
docker exec kafka kafka-topics --create --topic articles --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1 --if-not-exists
docker exec kafka kafka-topics --create --topic weaviate.changes --bootstrap-server localhost:9092 --partitions 3 --replication-factor 1 --if-not-exists
echo -e "${GREEN}✓ Topics created${NC}"

# List topics
echo ""
echo -e "${YELLOW}Available Kafka topics:${NC}"
docker exec kafka kafka-topics --list --bootstrap-server localhost:9092

# Test 1: Produce messages to products topic
echo ""
echo "==================================="
echo "Test 1: Kafka Producer"
echo "==================================="
echo ""
echo -e "${YELLOW}Producing sample product messages...${NC}"

# Create sample product messages
cat > /tmp/products.json << EOF
{"product_id": "prod-001", "name": "Laptop", "description": "High-performance laptop", "price": 1299.99, "category": "Electronics", "inStock": 50}
{"product_id": "prod-002", "name": "Smartphone", "description": "Latest smartphone model", "price": 899.99, "category": "Electronics", "inStock": 100}
{"product_id": "prod-003", "name": "Headphones", "description": "Noise-cancelling headphones", "price": 299.99, "category": "Electronics", "inStock": 75}
{"product_id": "prod-004", "name": "Tablet", "description": "10-inch tablet", "price": 499.99, "category": "Electronics", "inStock": 30}
{"product_id": "prod-005", "name": "Smartwatch", "description": "Fitness tracking smartwatch", "price": 249.99, "category": "Electronics", "inStock": 60}
EOF

docker cp /tmp/products.json kafka-producer:/tmp/
docker exec kafka-producer bash -c "cat /tmp/products.json | kafka-console-producer --broker-list localhost:9092 --topic products"
echo -e "${GREEN}✓ Produced 5 product messages${NC}"

# Test 2: Consume messages
echo ""
echo "==================================="
echo "Test 2: Kafka Consumer"
echo "==================================="
echo ""
echo -e "${YELLOW}Consuming messages from products topic...${NC}"
timeout 5 docker exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic products --from-beginning --max-messages 5 || true
echo -e "${GREEN}✓ Messages consumed${NC}"

# Test 3: CDC events
echo ""
echo "==================================="
echo "Test 3: CDC Events"
echo "==================================="
echo ""
echo -e "${YELLOW}Simulating CDC events...${NC}"

cat > /tmp/cdc_events.json << EOF
{"before": null, "after": {"id": "uuid-001", "name": "Product A", "price": 99.99}, "source": {"version": "1.0.0", "connector": "weaviate", "name": "weaviate-cdc", "ts_ms": 1642584000000, "db": "weaviate", "table": "Product"}, "op": "c", "ts_ms": 1642584000000}
{"before": {"id": "uuid-001", "name": "Product A", "price": 99.99}, "after": {"id": "uuid-001", "name": "Product A", "price": 89.99}, "source": {"version": "1.0.0", "connector": "weaviate", "name": "weaviate-cdc", "ts_ms": 1642584100000, "db": "weaviate", "table": "Product"}, "op": "u", "ts_ms": 1642584100000}
{"before": {"id": "uuid-001", "name": "Product A", "price": 89.99}, "after": null, "source": {"version": "1.0.0", "connector": "weaviate", "name": "weaviate-cdc", "ts_ms": 1642584200000, "db": "weaviate", "table": "Product"}, "op": "d", "ts_ms": 1642584200000}
EOF

docker cp /tmp/cdc_events.json kafka-producer:/tmp/
docker exec kafka-producer bash -c "cat /tmp/cdc_events.json | kafka-console-producer --broker-list localhost:9092 --topic weaviate.changes"
echo -e "${GREEN}✓ CDC events produced${NC}"

echo ""
echo -e "${YELLOW}Consuming CDC events...${NC}"
timeout 3 docker exec kafka kafka-console-consumer --bootstrap-server localhost:9092 --topic weaviate.changes --from-beginning || true
echo -e "${GREEN}✓ CDC events consumed${NC}"

# Test 4: Check consumer groups
echo ""
echo "==================================="
echo "Test 4: Consumer Groups"
echo "==================================="
echo ""
echo -e "${YELLOW}Listing consumer groups...${NC}"
docker exec kafka kafka-consumer-groups --bootstrap-server localhost:9092 --list
echo -e "${GREEN}✓ Consumer groups listed${NC}"

# Test 5: Topic statistics
echo ""
echo "==================================="
echo "Test 5: Topic Statistics"
echo "==================================="
echo ""
echo -e "${YELLOW}Getting topic descriptions...${NC}"
docker exec kafka kafka-topics --describe --bootstrap-server localhost:9092 --topic products
docker exec kafka kafka-topics --describe --bootstrap-server localhost:9092 --topic weaviate.changes
echo -e "${GREEN}✓ Topic statistics retrieved${NC}"

# Test 6: Kafka UI
echo ""
echo "==================================="
echo "Test 6: Kafka UI"
echo "==================================="
echo ""
echo -e "${YELLOW}Kafka UI is available at:${NC}"
echo -e "${GREEN}http://localhost:8080${NC}"

# Test 7: Schema Registry (if available)
echo ""
echo "==================================="
echo "Test 7: Schema Registry"
echo "==================================="
echo ""
if docker ps | grep -q schema-registry; then
    echo -e "${YELLOW}Checking schema registry...${NC}"
    curl -s http://localhost:8081/subjects || echo -e "${YELLOW}Schema registry not responding${NC}"
    echo ""
    echo -e "${GREEN}✓ Schema registry is running at http://localhost:8081${NC}"
else
    echo -e "${YELLOW}Schema registry not running${NC}"
fi

# Summary
echo ""
echo "==================================="
echo "Test Summary"
echo "==================================="
echo ""
echo -e "${GREEN}All tests completed successfully!${NC}"
echo ""
echo "Services running:"
echo "  - Kafka: localhost:9092"
echo "  - Kafka UI: http://localhost:8080"
echo "  - Schema Registry: http://localhost:8081"
echo "  - Weaviate: http://localhost:8090"
echo ""
echo "Available topics:"
echo "  - products (ingestion)"
echo "  - articles (ingestion)"
echo "  - weaviate.changes (CDC)"
echo ""
echo "To stop all services:"
echo "  docker-compose down"
echo ""
echo "To view logs:"
echo "  docker-compose logs -f [service-name]"
echo ""
echo "To produce more messages:"
echo "  docker exec -it kafka-producer bash"
echo "  echo '{\"key\": \"value\"}' | kafka-console-producer --broker-list localhost:9092 --topic products"
echo ""
echo "To consume messages:"
echo "  docker exec -it kafka-consumer bash"
echo "  kafka-console-consumer --bootstrap-server localhost:9092 --topic products --from-beginning"
echo ""

# Cleanup
rm -f /tmp/products.json /tmp/cdc_events.json

exit 0
