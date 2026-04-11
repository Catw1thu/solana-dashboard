FROM golang:1.25-bookworm AS build

WORKDIR /src/solana-dashboard-go

COPY solana-dashboard-go/go.mod solana-dashboard-go/go.sum ./
RUN go mod download

COPY solana-dashboard-go/ ./
RUN CGO_ENABLED=0 GOOS=linux go build -trimpath -ldflags="-s -w" -o /out/dashboard-api ./cmd/api

FROM debian:bookworm-slim

RUN apt-get update \
    && apt-get install -y --no-install-recommends ca-certificates \
    && rm -rf /var/lib/apt/lists/*

WORKDIR /app
COPY --from=build /out/dashboard-api /usr/local/bin/dashboard-api

EXPOSE 8080

ENTRYPOINT ["/usr/local/bin/dashboard-api"]
