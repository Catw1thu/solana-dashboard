package metadatafetcher

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"solana-dashboard-go/internal/db"
	"solana-dashboard-go/internal/events"
	serviceeventpb "solana-dashboard-go/internal/gen/serviceeventpb"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	streamName      = "SERVICE_EVENTS"
	streamSubject   = "solana.tracked.>"
	durableConsumer = "metadata_fetcher"
	fetchBatchSize  = 16
	fetchMaxWait    = 1 * time.Second
	httpTimeout     = 10 * time.Second
)

type Consumer struct {
	natsURL    string
	db         *db.DB
	httpClient *http.Client
}

type MetadataResponse struct {
	Name        string `json:"name"`
	Symbol      string `json:"symbol"`
	Description string `json:"description"`
	Image       string `json:"image"`
}

func NewConsumer(natsURL string, database *db.DB) *Consumer {
	return &Consumer{
		natsURL: natsURL,
		db:      database,
		httpClient: &http.Client{
			Timeout: httpTimeout,
		},
	}
}

func (c *Consumer) Run(ctx context.Context) error {
	nc, err := nats.Connect(c.natsURL)
	if err != nil {
		return fmt.Errorf("connect nats: %w", err)
	}
	defer nc.Drain()

	js, err := nc.JetStream()
	if err != nil {
		return fmt.Errorf("create jetstream context: %w", err)
	}

	// We assume main consumer already ensures the stream, but play it safe
	if _, err := js.StreamInfo(streamName); err != nil {
		return fmt.Errorf("stream %s not found: %w", streamName, err)
	}

	sub, err := js.PullSubscribe(
		streamSubject,
		durableConsumer,
		nats.BindStream(streamName),
		nats.ManualAck(),
	)
	if err != nil {
		return fmt.Errorf("create pull subscription for metadata_fetcher: %w", err)
	}

	log.Printf("metadata_fetcher consumer listening on %s subject %s", streamName, streamSubject)

	for {
		select {
		case <-ctx.Done():
			return nil
		default:
		}

		messages, err := sub.Fetch(fetchBatchSize, nats.MaxWait(fetchMaxWait))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			return fmt.Errorf("fetch jetstream messages: %w", err)
		}

		for _, message := range messages {
			if err := c.handleMessage(ctx, message); err != nil {
				log.Printf("metadata_fetcher consume error: %v", err)
				if nakErr := message.Nak(); nakErr != nil {
					log.Printf("metadata_fetcher nak error: %v", nakErr)
				}
				continue
			}

			if err := message.Ack(); err != nil {
				log.Printf("metadata_fetcher ack error: %v", err)
			}
		}
	}
}

func (c *Consumer) handleMessage(ctx context.Context, message *nats.Msg) error {
	var protoEvent serviceeventpb.EventEnvelope
	if err := proto.Unmarshal(message.Data, &protoEvent); err != nil {
		// Do not block queue for unmarshal errors
		log.Printf("unmarshal protobuf event error: %v", err)
		return nil
	}

	decoded, err := events.DecodedEnvelopeFromProto(&protoEvent)
	if err != nil {
		return nil // skip silently
	}

	// Filter and process only metadata-rich creation payloads
	switch p := decoded.Payload.(type) {
	case events.PumpfunCreatePayload:
		if p.URI != "" {
			go c.processMetadata(p.Mint, p.URI)
		}
	}

	// We treat messages as handled efficiently; heavy lifting is in background processMetadata.
	// We don't nak them to retry since processMetadata does its own bounded fetching.
	// If strong consistency is desired, `processMetadata` should block, but for IPFS it takes seconds.
	return nil
}

func (c *Consumer) processMetadata(mint, rawUri string) {
	uri := resolveIPFSURI(rawUri)

	// fmt.Printf("[metadata_fetcher] fetching URI for mint=%s uri=%s\n", mint, uri)

	req, err := http.NewRequest(http.MethodGet, uri, nil)
	if err != nil {
		log.Printf("[metadata_fetcher] error creating request: %v", err)
		return
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		log.Printf("[metadata_fetcher] error fetching uri %s: %v", uri, err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		log.Printf("[metadata_fetcher] bad status %d for uri %s", resp.StatusCode, uri)
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("[metadata_fetcher] error reading body from %s: %v", uri, err)
		return
	}

	var meta MetadataResponse
	if err := json.Unmarshal(body, &meta); err != nil {
		log.Printf("[metadata_fetcher] error parsing json from %s: %v", uri, err)
		return
	}

	if meta.Image == "" {
		return
	}

	resolvedImage := resolveIPFSURI(meta.Image)

	_, err = c.db.Pool.Exec(context.Background(), `
		UPDATE token_metadata_current 
		SET image_uri = $1 
		WHERE mint = $2
	`, resolvedImage, mint)

	if err != nil {
		log.Printf("[metadata_fetcher] error updating db for mint %s: %v", mint, err)
		return
	}
}

func resolveIPFSURI(uri string) string {
	if strings.HasPrefix(uri, "ipfs://") {
		return strings.Replace(uri, "ipfs://", "https://ipfs.io/ipfs/", 1)
	}
	return uri
}
