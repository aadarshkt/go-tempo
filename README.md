# Install golang-migrate if you don't have it
brew install golang-migrate

# Run migrations
migrate -path migrations -database "postgresql://postgres:postgres@localhost:5432/workflow_db?sslmode=disable" up

cd /Users/aadarshkt/Desktop/Projects/go-tempo
go run cmd/server/main.go

# Start only Postgres and Redis
docker-compose up postgres redis -d

docker exec workflow_redis redis-cli LLEN workflow:queue:pending

docker exec workflow_redis redis-cli LRANGE workflow:queue:pending

# Monitor tasks in redis queue
docker exec workflow_redis redis-cli MONITOR

# Check tables in docker container