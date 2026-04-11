package jetstream

import (
	"context"
	"errors"
	"fmt"
	"log"
	"time"

	"solana-dashboard-go/internal/events"
	serviceeventpb "solana-dashboard-go/internal/gen/serviceeventpb"
	"solana-dashboard-go/internal/ingest"
	"solana-dashboard-go/internal/observability"

	"github.com/nats-io/nats.go"
	"google.golang.org/protobuf/proto"
)

const (
	streamName      = "SERVICE_EVENTS"
	streamSubject   = "solana.tracked.>"
	durableConsumer = "dashboard_api"
	fetchBatchSize  = 64
	fetchMaxWait    = 1 * time.Second
	streamMaxAge    = 7 * 24 * time.Hour
)

type Consumer struct {
	natsURL string
	service *ingest.Service
}

func NewConsumer(natsURL string, service *ingest.Service) *Consumer {
	return &Consumer{
		natsURL: natsURL,
		service: service,
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

	if err := ensureStream(js); err != nil {
		return err
	}

	sub, err := js.PullSubscribe(
		streamSubject,
		durableConsumer,
		nats.BindStream(streamName),
		nats.ManualAck(),
	)
	if err != nil {
		return fmt.Errorf("create pull subscription: %w", err)
	}

	log.Printf("jetstream consumer listening on %s subject %s", streamName, streamSubject)

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
				observability.Default().IncCounter("jetstream_consume_errors_total", 1)
				log.Printf("jetstream consume error: %v", err)
				if nakErr := message.Nak(); nakErr != nil {
					log.Printf("jetstream nak error: %v", nakErr)
				}
				continue
			}

			if meta, err := message.Metadata(); err == nil && meta != nil {
				observability.Default().SetGauge("jetstream_pending_messages", int64(meta.NumPending))
				observability.Default().SetGauge("jetstream_last_stream_sequence", int64(meta.Sequence.Stream))
				observability.Default().SetGauge("jetstream_last_consumer_sequence", int64(meta.Sequence.Consumer))
			}
			observability.Default().IncCounter("jetstream_messages_total", 1)

			if err := message.Ack(); err != nil {
				log.Printf("jetstream ack error: %v", err)
			}
		}
	}
}

func ensureStream(js nats.JetStreamContext) error {
	if _, err := js.StreamInfo(streamName); err == nil {
		return nil
	}

	_, err := js.AddStream(&nats.StreamConfig{
		Name:     streamName,
		Subjects: []string{streamSubject},
		Storage:  nats.FileStorage,
		MaxAge:   streamMaxAge,
	})
	if err != nil {
		return fmt.Errorf("ensure jetstream stream: %w", err)
	}

	return nil
}

func (c *Consumer) handleMessage(ctx context.Context, message *nats.Msg) error {
	var protoEvent serviceeventpb.EventEnvelope
	if err := proto.Unmarshal(message.Data, &protoEvent); err != nil {
		return fmt.Errorf("unmarshal protobuf event: %w", err)
	}

	decoded, err := events.DecodedEnvelopeFromProto(&protoEvent)
	if err != nil {
		return fmt.Errorf("convert protobuf event: %w", err)
	}

	if err := c.service.HandleDecodedEvent(ctx, decoded); err != nil {
		return fmt.Errorf("handle event %s: %w", decoded.Envelope.EventID, err)
	}

	return nil
}
