# Image-Based Deployment

This deployment mode keeps source code off the server.

## 1. Build and push images from your build machine

```bash
export GO_IMAGE=ghcr.io/your-org/solana-dashboard-go:latest
export LAB_IMAGE=ghcr.io/your-org/solana-dashboard-lab:latest
export MIGRATE_IMAGE=ghcr.io/your-org/solana-dashboard-migrate:latest

docker build -f docker/Dockerfile.go -t "$GO_IMAGE" .
docker build -f docker/Dockerfile.rust -t "$LAB_IMAGE" .
docker build -f docker/Dockerfile.migrate -t "$MIGRATE_IMAGE" .

docker push "$GO_IMAGE"
docker push "$LAB_IMAGE"
docker push "$MIGRATE_IMAGE"
```

## 2. Copy only deployment files to the server

Files needed on the server:

- `docker-compose.deploy.yml`
- `.env.deploy`

## 3. Start services on the server

```bash
mkdir -p data/postgres data/redis data/nats
cp .env.deploy.example .env.deploy
vim .env.deploy

docker compose -f docker-compose.deploy.yml --env-file .env.deploy up -d
```

## 4. Useful commands

```bash
docker compose -f docker-compose.deploy.yml --env-file .env.deploy ps
docker compose -f docker-compose.deploy.yml --env-file .env.deploy logs -f dashboard-go dashboard-lab
docker compose -f docker-compose.deploy.yml --env-file .env.deploy pull
docker compose -f docker-compose.deploy.yml --env-file .env.deploy up -d
docker compose -f docker-compose.deploy.yml --env-file .env.deploy down
```
