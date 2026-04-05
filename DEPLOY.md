# Deploy With GitHub-Built Images

This setup keeps source code off the server. GitHub Actions builds and publishes images to GHCR, and the server only pulls images plus a small compose bundle.

## Repositories and responsibilities

- `solana-dashboard-go`
  - Publishes `solana-dashboard-go`
  - Publishes `solana-dashboard-migrate`
  - Owns `docker-compose.deploy.yml`
- `solana-dashboard-lab`
  - Publishes `solana-dashboard-lab`

## 1. Local setup

In `solana-dashboard-go`:

```bash
go test ./...
docker build -f docker/Dockerfile.api -t ghcr.io/<org>/solana-dashboard-go:local .
docker build -f docker/Dockerfile.migrate -t ghcr.io/<org>/solana-dashboard-migrate:local .
```

In `solana-dashboard-lab`:

```bash
cargo test
docker build -f docker/Dockerfile.rust -t ghcr.io/<org>/solana-dashboard-lab:local .
```

Then commit and push both repos to GitHub.

## 2. GitHub setup

### In the `solana-dashboard-go` repo

- Commit `.github/workflows/publish-images.yml`
- The workflow publishes:
  - `ghcr.io/<owner>/solana-dashboard-go`
  - `ghcr.io/<owner>/solana-dashboard-migrate`

### In the `solana-dashboard-lab` repo

- Commit `.github/workflows/publish-image.yml`
- The workflow publishes:
  - `ghcr.io/<owner>/solana-dashboard-lab`

### GitHub permissions

- Make sure GitHub Actions is enabled in both repos.
- If GHCR packages are private, the server will need a PAT with `read:packages`.
- If your org has strict Actions policy, allow workflows to write packages.

Useful references:

- https://docs.github.com/en/actions/tutorials/publishing-packages/publishing-docker-images
- https://docs.github.com/en/packages/guides/about-github-container-registry
- https://docs.github.com/en/actions/sharing-automations/reusing-workflows

## 3. Server setup

Only copy these files from `solana-dashboard-go` to the server:

- `docker-compose.deploy.yml`
- `.env.deploy.example`

Then on the server:

```bash
mkdir -p /srv/solana-dashboard
cd /srv/solana-dashboard
mkdir -p data/postgres data/redis data/nats
cp /path/to/.env.deploy.example .env.deploy
vim .env.deploy
```

If GHCR images are private:

```bash
echo "$GHCR_PAT" | docker login ghcr.io -u <github-user> --password-stdin
```

Start the stack:

```bash
docker compose -f docker-compose.deploy.yml --env-file .env.deploy pull
docker compose -f docker-compose.deploy.yml --env-file .env.deploy up -d
```

Check status:

```bash
docker compose -f docker-compose.deploy.yml --env-file .env.deploy ps
docker compose -f docker-compose.deploy.yml --env-file .env.deploy logs -f dashboard-go dashboard-lab
```

Update to a new image version:

```bash
vim .env.deploy
docker compose -f docker-compose.deploy.yml --env-file .env.deploy pull
docker compose -f docker-compose.deploy.yml --env-file .env.deploy up -d
```

## 4. Recommended tagging flow

- On `main`, GitHub Actions will publish `latest` and `sha-<commit>`.
- For production, pin the server to `sha-<commit>` or a release tag instead of `latest`.
