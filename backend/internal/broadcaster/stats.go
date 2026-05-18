package broadcaster

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/observability"
	"solana-dashboard-go/internal/realtime"
	"solana-dashboard-go/internal/store"

	"github.com/nats-io/nats.go"
)

const (
	wrappedSolMint       = "So11111111111111111111111111111111111111112"
	solDecimals          = 9
	defaultTokenDecimals = 6
	shortWindowSeconds   = int64(300)
	longWindowTTLSeconds = int64(24 * 60 * 60)
	globalHubBufferSize  = 8192
	backfillRepairLimit  = 250
)

type TokenStatsPayload struct {
	Mint      string  `json:"mint"`
	Price     float64 `json:"p"`
	Timestamp int64   `json:"t"`

	P1m  float64 `json:"p1m"`
	B1m  int     `json:"b1m"`
	S1m  int     `json:"s1m"`
	BV1m float64 `json:"bv1m"`
	SV1m float64 `json:"sv1m"`
	V1m  float64 `json:"v1m"`

	P5m  float64 `json:"p5m"`
	B5m  int     `json:"b5m"`
	S5m  int     `json:"s5m"`
	BV5m float64 `json:"bv5m"`
	SV5m float64 `json:"sv5m"`
	V5m  float64 `json:"v5m"`

	P1h  float64 `json:"p1h"`
	B1h  int     `json:"b1h"`
	S1h  int     `json:"s1h"`
	BV1h float64 `json:"bv1h"`
	SV1h float64 `json:"sv1h"`
	V1h  float64 `json:"v1h"`

	P4h  float64 `json:"p4h"`
	B4h  int     `json:"b4h"`
	S4h  int     `json:"s4h"`
	BV4h float64 `json:"bv4h"`
	SV4h float64 `json:"sv4h"`
	V4h  float64 `json:"v4h"`

	P24h  float64 `json:"p24h"`
	B24h  int     `json:"b24h"`
	S24h  int     `json:"s24h"`
	BV24h float64 `json:"bv24h"`
	SV24h float64 `json:"sv24h"`
	V24h  float64 `json:"v24h"`
}

type ShortTrade struct {
	Timestamp int64
	IsBuy     bool
	Volume    float64
	Price     float64
}

type MinuteBucket struct {
	BucketStart int64
	Buys        int
	Sells       int
	Volume      float64
	BuyVolume   float64
	SellVolume  float64
	OpenPrice   float64
	ClosePrice  float64
}

type legacyWindow struct {
	LastPrice     float64
	LastUnixTs    int64
	FirstPrice    float64
	ShortTrades   []ShortTrade
	MinuteBuckets [1440]MinuteBucket
}

type SecondBucket struct {
	BucketStart int64
	Buys        int64
	Sells       int64
	Volume      float64
	BuyVolume   float64
	SellVolume  float64
	OpenPrice   float64
	ClosePrice  float64
}

type TokenWindowState struct {
	mu                  sync.Mutex
	Seeded              bool
	LastPrice           float64
	LastUnixTs          int64
	LastActiveTs        int64
	LastPersistedMinute int64
	LastLogID           int64
	SecondBuckets       [300]SecondBucket
}

type Broadcaster struct {
	nc              *nats.Conn
	hub             *realtime.Hub
	metricsStore    tradeMetricsStore
	windows         map[string]*TokenWindowState
	globalHubBuffer int
	mu              sync.RWMutex
}

type tradeMetricsStore interface {
	ListTradeMetricsForStatsByMint(ctx context.Context, mint string) ([]store.TradeMetricPoint, error)
	ListRecentTradeMetricsByMint(ctx context.Context, mint string, sinceUnix int64) ([]store.TradeMetricPoint, error)
	ListLongWindowMetricsByMints(ctx context.Context, mints []string, nowTs int64) (map[string]store.LongWindowMetricsRecord, error)
	ListMetricBackfillCandidates(ctx context.Context, limit int) ([]string, error)
	ListLatestTradeSeedsByMints(ctx context.Context, mints []string) (map[string]store.LatestTradeSeedRecord, error)
	UpsertTokenMetricsCurrent(ctx context.Context, records []store.TokenMetricsCurrentRecord) error
	LoadTokenMetricsCurrentByMint(ctx context.Context, mint string) (*store.TokenMetricsCurrentRecord, error)
}

type windowSnapshot struct {
	AnchorPrice *float64
	Buys        int64
	Sells       int64
	BuyVolume   float64
	SellVolume  float64
	Volume      float64
}

func NewBroadcaster(natsURL string, hub *realtime.Hub, metricsStore tradeMetricsStore) *Broadcaster {
	var nc *nats.Conn
	if natsURL != "" {
		conn, err := nats.Connect(natsURL)
		if err != nil {
			log.Printf("[stats] failed to connect to nats: %v", err)
		} else {
			nc = conn
		}
	}

	return &Broadcaster{
		nc:              nc,
		hub:             hub,
		metricsStore:    metricsStore,
		windows:         make(map[string]*TokenWindowState),
		globalHubBuffer: globalHubBufferSize,
	}
}

func (b *Broadcaster) BackfillCurrentMetrics(ctx context.Context, limit int) error {
	if b == nil || b.metricsStore == nil || limit <= 0 {
		return nil
	}

	start := time.Now()
	mints, err := b.metricsStore.ListMetricBackfillCandidates(ctx, limit)
	if err != nil {
		observability.Default().IncCounter("metrics_backfill_errors_total", 1)
		return err
	}
	if len(mints) == 0 {
		return nil
	}

	nowTs := time.Now().Unix()
	longMetrics, err := b.metricsStore.ListLongWindowMetricsByMints(ctx, mints, nowTs)
	if err != nil {
		observability.Default().IncCounter("metrics_backfill_errors_total", 1)
		return err
	}
	latestSeeds, err := b.metricsStore.ListLatestTradeSeedsByMints(ctx, mints)
	if err != nil {
		observability.Default().IncCounter("metrics_backfill_errors_total", 1)
		return err
	}

	records := make([]store.TokenMetricsCurrentRecord, 0, len(mints))
	for _, mint := range mints {
		window := &TokenWindowState{Seeded: true}

		recentMetrics, err := b.metricsStore.ListRecentTradeMetricsByMint(ctx, mint, nowTs-shortWindowSeconds)
		if err != nil {
			observability.Default().IncCounter("metrics_backfill_errors_total", 1)
			return err
		}
		for _, metric := range recentMetrics {
			appendSecondBucket(window, metric.Side == "buy", metric.Volume, metric.Price, metric.EventUnixTS, 0)
		}

		if seed, ok := latestSeeds[mint]; ok {
			if seed.LatestPrice != nil {
				window.LastPrice = *seed.LatestPrice
			}
			if seed.LatestEventUnix != nil {
				window.LastUnixTs = *seed.LatestEventUnix
				window.LastActiveTs = *seed.LatestEventUnix
			}
		}

		record, ok := b.buildCurrentRecord(mint, window, longMetrics[mint], nowTs)
		if !ok {
			continue
		}
		records = append(records, record)
	}

	if len(records) == 0 {
		return nil
	}
	if err := b.metricsStore.UpsertTokenMetricsCurrent(ctx, records); err != nil {
		observability.Default().IncCounter("metrics_backfill_errors_total", 1)
		return err
	}

	observability.Default().IncCounter("metrics_backfill_runs_total", 1)
	observability.Default().IncCounter("metrics_backfill_records_total", int64(len(records)))
	observability.Default().ObserveDuration("metrics_backfill_latency_ms", time.Since(start))
	return nil
}

func BuildPayloadFromCurrentMetrics(mint string, metrics *store.TokenMetricsCurrentRecord) (*TokenStatsPayload, bool) {
	if metrics == nil || metrics.LatestPrice == nil || metrics.LatestEventUnix == nil {
		return nil, false
	}

	payload := &TokenStatsPayload{
		Mint:      mint,
		Price:     *metrics.LatestPrice,
		Timestamp: *metrics.LatestEventUnix,
		P1m:       derefAnchor(metrics.AnchorPrice1m),
		P5m:       derefAnchor(metrics.AnchorPrice5m),
		P1h:       derefAnchor(metrics.AnchorPrice1h),
		P4h:       derefAnchor(metrics.AnchorPrice4h),
		P24h:      derefAnchor(metrics.AnchorPrice24h),
		B1m:       int(metrics.Buys1m),
		S1m:       int(metrics.Sells1m),
		BV1m:      metrics.BuyVolume1m,
		SV1m:      metrics.SellVolume1m,
		V1m:       metrics.Volume1m,
		B5m:       int(metrics.Buys5m),
		S5m:       int(metrics.Sells5m),
		BV5m:      metrics.BuyVolume5m,
		SV5m:      metrics.SellVolume5m,
		V5m:       metrics.Volume5m,
		B1h:       int(metrics.Buys1h),
		S1h:       int(metrics.Sells1h),
		BV1h:      metrics.BuyVolume1h,
		SV1h:      metrics.SellVolume1h,
		V1h:       metrics.Volume1h,
		B4h:       int(metrics.Buys4h),
		S4h:       int(metrics.Sells4h),
		BV4h:      metrics.BuyVolume4h,
		SV4h:      metrics.SellVolume4h,
		V4h:       metrics.Volume4h,
		B24h:      int(metrics.Buys24h),
		S24h:      int(metrics.Sells24h),
		BV24h:     metrics.BuyVolume24h,
		SV24h:     metrics.SellVolume24h,
		V24h:      metrics.Volume24h,
	}
	return payload, true
}

func BuildPayloadFromTradeMetrics(mint string, metrics []store.TradeMetricPoint, nowTs int64) (TokenStatsPayload, bool) {
	if len(metrics) == 0 {
		return TokenStatsPayload{}, false
	}

	window := &legacyWindow{}
	seedLegacyWindow(window, metrics, nowTs)
	if window.LastPrice <= 0 {
		return TokenStatsPayload{}, false
	}
	return generateLegacyPayload(mint, window, nowTs), true
}

func derefAnchor(value *float64) float64 {
	if value == nil {
		return 0
	}
	return *value
}

func (b *Broadcaster) getWindow(mint string) *TokenWindowState {
	b.mu.RLock()
	w, ok := b.windows[mint]
	b.mu.RUnlock()
	if ok {
		return w
	}

	b.mu.Lock()
	defer b.mu.Unlock()
	w, ok = b.windows[mint]
	if !ok {
		w = &TokenWindowState{}
		b.windows[mint] = w
	}
	return w
}

func (b *Broadcaster) removeWindow(mint string) {
	b.mu.Lock()
	delete(b.windows, mint)
	b.mu.Unlock()
}

func (b *Broadcaster) windowCount() int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.windows)
}

func (b *Broadcaster) handleTrade(env *events.Envelope) {
	if env == nil {
		return
	}

	switch {
	case isPumpfunTradeEvent(env):
		b.handlePumpfunTrade(env)
	case isPumpAmmTradeEvent(env):
		b.handlePumpAmmTrade(env)
	}
}

func (b *Broadcaster) handlePumpfunTrade(env *events.Envelope) {
	bytes, _ := json.Marshal(env.Payload)
	var payload events.PumpfunTradePayload
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return
	}

	solAmt, _ := strconv.ParseFloat(payload.SolAmount, 64)
	vol := scaleAmount(solAmt, solDecimals)

	tokenAmt, _ := strconv.ParseFloat(payload.TokenAmount, 64)
	tokenQty := scaleAmount(tokenAmt, defaultTokenDecimals)

	price := 0.0
	if tokenQty > 0 {
		price = vol / tokenQty
	}

	b.ensureWindowSeeded(payload.Mint, env.EventUnixTS)
	b.recordTrade(payload.Mint, payload.Side == "buy", vol, price, env.EventUnixTS, env.LogID)
}

func (b *Broadcaster) handlePumpAmmTrade(env *events.Envelope) {
	bytes, _ := json.Marshal(env.Payload)
	var payload events.PumpAmmSwapPayload
	if err := json.Unmarshal(bytes, &payload); err != nil {
		return
	}

	mint, isBuy, quoteAmountRaw, tokenAmountRaw, ok := pumpAmmTradeMetrics(env, payload)
	if !ok {
		return
	}

	quoteAmt, err := strconv.ParseFloat(quoteAmountRaw, 64)
	if err != nil {
		return
	}
	tokenAmt, err := strconv.ParseFloat(tokenAmountRaw, 64)
	if err != nil {
		return
	}

	vol := scaleAmount(quoteAmt, solDecimals)
	tokenQty := scaleAmount(tokenAmt, defaultTokenDecimals)
	price := 0.0
	if tokenQty > 0 {
		price = vol / tokenQty
	}

	b.ensureWindowSeeded(mint, env.EventUnixTS)
	b.recordTrade(mint, isBuy, vol, price, env.EventUnixTS, env.LogID)
}

func (b *Broadcaster) ensureWindowSeeded(mint string, nowTs int64) {
	if b.metricsStore == nil || mint == "" {
		return
	}

	window := b.getWindow(mint)
	window.mu.Lock()
	seeded := window.Seeded
	window.mu.Unlock()
	if seeded {
		return
	}

	sinceUnix := nowTs - shortWindowSeconds
	metrics, err := b.metricsStore.ListRecentTradeMetricsByMint(context.Background(), mint, sinceUnix)
	if err != nil {
		observability.Default().IncCounter("stats_seed_errors_total", 1)
		log.Printf("[stats] failed to seed short window for mint=%s: %v", mint, err)
		return
	}

	window.mu.Lock()
	defer window.mu.Unlock()
	if window.Seeded {
		return
	}
	for _, metric := range metrics {
		appendSecondBucket(window, metric.Side == "buy", metric.Volume, metric.Price, metric.EventUnixTS, 0)
	}
	window.Seeded = true
	observability.Default().IncCounter("stats_seeded_windows_total", 1)
}

func (b *Broadcaster) recordTrade(mint string, isBuy bool, vol float64, price float64, ts int64, logID int64) {
	if mint == "" {
		return
	}
	window := b.getWindow(mint)
	window.mu.Lock()
	defer window.mu.Unlock()

	appendSecondBucket(window, isBuy, vol, price, ts, logID)
	observability.Default().IncCounter("stats_recorded_trades_total", 1)
}

func appendSecondBucket(window *TokenWindowState, isBuy bool, vol float64, price float64, ts int64, logID int64) {
	window.LastPrice = price
	window.LastUnixTs = ts
	window.LastActiveTs = ts
	if logID > window.LastLogID {
		window.LastLogID = logID
	}

	idx := int(ts % int64(len(window.SecondBuckets)))
	if idx < 0 {
		idx += len(window.SecondBuckets)
	}
	bucketStart := ts
	bucket := &window.SecondBuckets[idx]
	if bucket.BucketStart != bucketStart {
		*bucket = SecondBucket{
			BucketStart: bucketStart,
			OpenPrice:   price,
			ClosePrice:  price,
		}
	} else if bucket.OpenPrice == 0 {
		bucket.OpenPrice = price
	}
	bucket.ClosePrice = price
	if isBuy {
		bucket.Buys++
		bucket.BuyVolume += vol
	} else {
		bucket.Sells++
		bucket.SellVolume += vol
	}
	bucket.Volume += vol
}

func summarizeSecondBuckets(window *TokenWindowState, nowTs int64, secondsBack int64) windowSnapshot {
	cutoff := nowTs - secondsBack
	var snapshot windowSnapshot
	var earliestBucketTs int64

	for _, bucket := range window.SecondBuckets {
		if bucket.BucketStart <= cutoff || bucket.BucketStart > nowTs || bucket.ClosePrice == 0 {
			continue
		}
		snapshot.Buys += bucket.Buys
		snapshot.Sells += bucket.Sells
		snapshot.BuyVolume += bucket.BuyVolume
		snapshot.SellVolume += bucket.SellVolume
		snapshot.Volume += bucket.Volume
		if earliestBucketTs == 0 || bucket.BucketStart < earliestBucketTs {
			earliestBucketTs = bucket.BucketStart
			anchor := bucket.OpenPrice
			snapshot.AnchorPrice = &anchor
		}
	}

	return snapshot
}

func mergeWindowSnapshots(long windowSnapshot, tail windowSnapshot) windowSnapshot {
	result := windowSnapshot{
		AnchorPrice: long.AnchorPrice,
		Buys:        long.Buys + tail.Buys,
		Sells:       long.Sells + tail.Sells,
		BuyVolume:   long.BuyVolume + tail.BuyVolume,
		SellVolume:  long.SellVolume + tail.SellVolume,
		Volume:      long.Volume + tail.Volume,
	}
	if result.AnchorPrice == nil {
		result.AnchorPrice = tail.AnchorPrice
	}
	return result
}

func (b *Broadcaster) buildCurrentRecord(mint string, window *TokenWindowState, long store.LongWindowMetricsRecord, nowTs int64) (store.TokenMetricsCurrentRecord, bool) {
	if window.LastPrice <= 0 || window.LastUnixTs <= 0 {
		return store.TokenMetricsCurrentRecord{}, false
	}

	short1m := summarizeSecondBuckets(window, nowTs, 60)
	short5m := summarizeSecondBuckets(window, nowTs, shortWindowSeconds)

	long1h := mergeWindowSnapshots(
		windowSnapshot{
			AnchorPrice: long.AnchorPrice1h,
			Buys:        long.Buys1h,
			Sells:       long.Sells1h,
			BuyVolume:   long.BuyVolume1h,
			SellVolume:  long.SellVolume1h,
			Volume:      long.Volume1h,
		},
		short5m,
	)
	long4h := mergeWindowSnapshots(
		windowSnapshot{
			AnchorPrice: long.AnchorPrice4h,
			Buys:        long.Buys4h,
			Sells:       long.Sells4h,
			BuyVolume:   long.BuyVolume4h,
			SellVolume:  long.SellVolume4h,
			Volume:      long.Volume4h,
		},
		short5m,
	)
	long24h := mergeWindowSnapshots(
		windowSnapshot{
			AnchorPrice: long.AnchorPrice24h,
			Buys:        long.Buys24h,
			Sells:       long.Sells24h,
			BuyVolume:   long.BuyVolume24h,
			SellVolume:  long.SellVolume24h,
			Volume:      long.Volume24h,
		},
		short5m,
	)

	latestPrice := window.LastPrice
	latestEventUnix := window.LastUnixTs

	return store.TokenMetricsCurrentRecord{
		Mint:            mint,
		LatestPrice:     &latestPrice,
		LatestEventUnix: &latestEventUnix,
		AnchorPrice1m:   short1m.AnchorPrice,
		AnchorPrice5m:   short5m.AnchorPrice,
		AnchorPrice1h:   long1h.AnchorPrice,
		AnchorPrice4h:   long4h.AnchorPrice,
		AnchorPrice24h:  long24h.AnchorPrice,
		Txns1m:          short1m.Buys + short1m.Sells,
		Txns5m:          short5m.Buys + short5m.Sells,
		Txns1h:          long1h.Buys + long1h.Sells,
		Txns4h:          long4h.Buys + long4h.Sells,
		Txns24h:         long24h.Buys + long24h.Sells,
		Buys1m:          short1m.Buys,
		Buys5m:          short5m.Buys,
		Buys1h:          long1h.Buys,
		Buys4h:          long4h.Buys,
		Buys24h:         long24h.Buys,
		Sells1m:         short1m.Sells,
		Sells5m:         short5m.Sells,
		Sells1h:         long1h.Sells,
		Sells4h:         long4h.Sells,
		Sells24h:        long24h.Sells,
		Volume1m:        short1m.Volume,
		Volume5m:        short5m.Volume,
		Volume1h:        long1h.Volume,
		Volume4h:        long4h.Volume,
		Volume24h:       long24h.Volume,
		BuyVolume1m:     short1m.BuyVolume,
		BuyVolume5m:     short5m.BuyVolume,
		BuyVolume1h:     long1h.BuyVolume,
		BuyVolume4h:     long4h.BuyVolume,
		BuyVolume24h:    long24h.BuyVolume,
		SellVolume1m:    short1m.SellVolume,
		SellVolume5m:    short5m.SellVolume,
		SellVolume1h:    long1h.SellVolume,
		SellVolume4h:    long4h.SellVolume,
		SellVolume24h:   long24h.SellVolume,
		SourceLogID:     window.LastLogID,
	}, true
}

func (b *Broadcaster) Sweep() {
	start := time.Now()
	nowTs := time.Now().Unix()
	currentMinute := (nowTs / 60) * 60

	type refreshTarget struct {
		mint       string
		window     *TokenWindowState
		subscribed bool
	}

	b.mu.RLock()
	targets := make([]refreshTarget, 0, len(b.windows))
	evictions := make([]string, 0)
	for mint, window := range b.windows {
		window.mu.Lock()
		lastActiveTs := window.LastActiveTs
		lastPersistedMinute := window.LastPersistedMinute
		window.mu.Unlock()

		subscribed := b.hub != nil && b.hub.HasSubscribers("token:"+mint)
		if !subscribed && lastActiveTs > 0 && nowTs-lastActiveTs > longWindowTTLSeconds {
			evictions = append(evictions, mint)
			continue
		}

		shortActive := lastActiveTs > 0 && nowTs-lastActiveTs <= shortWindowSeconds
		needsMinuteRefresh := lastPersistedMinute != currentMinute
		if subscribed || shortActive || needsMinuteRefresh {
			targets = append(targets, refreshTarget{
				mint:       mint,
				window:     window,
				subscribed: subscribed,
			})
		}
	}
	b.mu.RUnlock()

	for _, mint := range evictions {
		b.removeWindow(mint)
	}
	observability.Default().IncCounter("stats_evicted_windows_total", int64(len(evictions)))
	observability.Default().SetGauge("stats_windows_total", int64(b.windowCount()))

	if len(targets) == 0 {
		observability.Default().SetGauge("stats_refresh_targets", 0)
		return
	}
	observability.Default().SetGauge("stats_refresh_targets", int64(len(targets)))

	mints := make([]string, 0, len(targets))
	for _, target := range targets {
		mints = append(mints, target.mint)
	}

	longMetrics := map[string]store.LongWindowMetricsRecord{}
	if b.metricsStore != nil {
		items, err := b.metricsStore.ListLongWindowMetricsByMints(context.Background(), mints, nowTs)
		if err != nil {
			observability.Default().IncCounter("stats_long_window_load_errors_total", 1)
			log.Printf("[stats] failed to load long window metrics: %v", err)
		} else {
			longMetrics = items
		}
	}

	records := make([]store.TokenMetricsCurrentRecord, 0, len(targets))
	type publishEvent struct {
		mint     string
		envelope events.Envelope
	}
	publishQueue := make([]publishEvent, 0, len(targets))

	for _, target := range targets {
		window := target.window
		window.mu.Lock()
		if target.subscribed && !window.Seeded {
			window.mu.Unlock()
			b.ensureWindowSeeded(target.mint, nowTs)
			window.mu.Lock()
		}

		record, ok := b.buildCurrentRecord(target.mint, window, longMetrics[target.mint], nowTs)
		window.LastPersistedMinute = currentMinute
		window.mu.Unlock()
		if !ok {
			continue
		}

		records = append(records, record)
		if target.subscribed && b.hub != nil {
			payload, ok := BuildPayloadFromCurrentMetrics(target.mint, &record)
			if !ok {
				continue
			}
			bs, err := json.Marshal(payload)
			if err != nil {
				continue
			}
			publishQueue = append(publishQueue, publishEvent{
				mint: target.mint,
				envelope: events.Envelope{
					EventType:   "token_stat",
					Payload:     json.RawMessage(bs),
					EventUnixTS: nowTs,
				},
			})
		}
	}

	if b.metricsStore != nil && len(records) > 0 {
		if err := b.metricsStore.UpsertTokenMetricsCurrent(context.Background(), records); err != nil {
			observability.Default().IncCounter("stats_upsert_current_errors_total", 1)
			log.Printf("[stats] failed to upsert token metrics current: %v", err)
		}
	}

	for _, item := range publishQueue {
		b.hub.Publish("token:"+item.mint, item.envelope)
	}
	observability.Default().IncCounter("stats_refreshed_mints_total", int64(len(records)))
	observability.Default().IncCounter("stats_published_token_stats_total", int64(len(publishQueue)))
	observability.Default().ObserveDuration("stats_sweep_latency_ms", time.Since(start))
}

func (b *Broadcaster) Run(ctx context.Context) error {
	hubSub, hubEvents, hubOverflow := b.subscribeGlobalHub()
	defer func() {
		if b.hub != nil && hubSub != nil {
			b.hub.Unsubscribe(hubSub)
		}
	}()

	if hubSub == nil && b.nc != nil {
		js, err := b.nc.JetStream()
		if err != nil {
			return err
		}

		_, err = js.Subscribe("solana.tracked.>", func(msg *nats.Msg) {
			var env events.Envelope
			if err := json.Unmarshal(msg.Data, &env); err == nil {
				b.handleTrade(&env)
			}
		}, nats.DeliverNew())
		if err != nil {
			return err
		}
	}

	resubscribeGlobal := func(reason string) {
		if b.hub == nil {
			return
		}
		observability.Default().IncCounter("stats_global_resubscribe_total", 1)
		log.Printf("[stats] resubscribing global hub feed after %s", reason)
		if hubSub != nil {
			b.hub.Unsubscribe(hubSub)
		}
		hubSub, hubEvents, hubOverflow = b.subscribeGlobalHub()
		if b.metricsStore != nil {
			if err := b.BackfillCurrentMetrics(context.Background(), backfillRepairLimit); err != nil {
				observability.Default().IncCounter("stats_global_repair_errors_total", 1)
				log.Printf("[stats] failed to repair metrics after global %s: %v", reason, err)
			} else {
				observability.Default().IncCounter("stats_global_repair_runs_total", 1)
			}
		}
	}

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-hubEvents:
			if !ok {
				resubscribeGlobal("events_closed")
				continue
			}
			b.handleTrade(&event)
		case _, ok := <-hubOverflow:
			if !ok {
				resubscribeGlobal("overflow_closed")
				continue
			}
			observability.Default().IncCounter("stats_global_overflow_total", 1)
			resubscribeGlobal("overflow")
		case <-ticker.C:
			b.Sweep()
		}
	}
}

func (b *Broadcaster) subscribeGlobalHub() (*realtime.Subscription, <-chan events.Envelope, <-chan struct{}) {
	if b == nil || b.hub == nil {
		return nil, nil, nil
	}
	buffer := b.globalHubBuffer
	if buffer <= 0 {
		buffer = globalHubBufferSize
	}
	sub := b.hub.Subscribe("global", buffer)
	observability.Default().IncCounter("stats_global_subscribe_total", 1)
	return sub, sub.Events, sub.Overflow
}

func isPumpfunTradeEvent(env *events.Envelope) bool {
	if env == nil {
		return false
	}
	if env.Protocol == "pumpfun" && env.EventType == "trade" {
		return true
	}
	return env.EventType == "pumpfun.trade"
}

func isPumpAmmTradeEvent(env *events.Envelope) bool {
	if env == nil {
		return false
	}
	return env.Protocol == "pumpamm" && env.EventType == "swap"
}

func scaleAmount(value float64, decimals int) float64 {
	switch decimals {
	case 9:
		return value / 1e9
	case 6:
		return value / 1e6
	default:
		return value / mathPow10(decimals)
	}
}

func mathPow10(decimals int) float64 {
	result := 1.0
	for i := 0; i < decimals; i++ {
		result *= 10
	}
	return result
}

func trackedMint(env *events.Envelope, baseMint string, quoteMint string) string {
	if env != nil && env.Refs.Mint != nil && *env.Refs.Mint != "" {
		return *env.Refs.Mint
	}
	switch {
	case baseMint == wrappedSolMint && quoteMint != "":
		return quoteMint
	case quoteMint == wrappedSolMint && baseMint != "":
		return baseMint
	default:
		return ""
	}
}

func pumpAmmTradeMetrics(env *events.Envelope, payload events.PumpAmmSwapPayload) (mint string, isBuy bool, quoteAmountRaw string, tokenAmountRaw string, ok bool) {
	mint = trackedMint(env, payload.BaseMint, payload.QuoteMint)
	if mint == "" {
		return "", false, "", "", false
	}

	if mint == payload.QuoteMint {
		switch payload.Side {
		case "sell":
			tokenAmountRaw = stringValue(payload.QuoteAmountOut)
			quoteAmountRaw = stringValue(payload.BaseAmountIn)
			isBuy = true
		case "buy", "buy_exact_quote_in":
			tokenAmountRaw = stringValue(payload.QuoteAmountIn)
			quoteAmountRaw = stringValue(payload.BaseAmountOut)
			isBuy = false
		default:
			return "", false, "", "", false
		}
	} else {
		switch payload.Side {
		case "sell":
			tokenAmountRaw = stringValue(payload.BaseAmountIn)
			quoteAmountRaw = stringValue(payload.QuoteAmountOut)
			isBuy = false
		case "buy", "buy_exact_quote_in":
			tokenAmountRaw = stringValue(payload.BaseAmountOut)
			quoteAmountRaw = stringValue(payload.QuoteAmountIn)
			isBuy = true
		default:
			return "", false, "", "", false
		}
	}

	if tokenAmountRaw == "" || quoteAmountRaw == "" {
		return "", false, "", "", false
	}

	return mint, isBuy, quoteAmountRaw, tokenAmountRaw, true
}

func stringValue(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func seedLegacyWindow(window *legacyWindow, metrics []store.TradeMetricPoint, nowTs int64) {
	if len(metrics) == 0 {
		return
	}

	cutoff5m := nowTs - shortWindowSeconds
	var latestOlder *ShortTrade

	for _, metric := range metrics {
		isBuy := metric.Side == "buy"
		appendLegacyTrade(window, isBuy, metric.Volume, metric.Price, metric.EventUnixTS)
		trade := ShortTrade{
			Timestamp: metric.EventUnixTS,
			IsBuy:     isBuy,
			Volume:    metric.Volume,
			Price:     metric.Price,
		}
		if metric.EventUnixTS >= cutoff5m {
			window.ShortTrades = append(window.ShortTrades, trade)
			continue
		}
		copyTrade := trade
		latestOlder = &copyTrade
	}

	if latestOlder != nil {
		window.ShortTrades = append([]ShortTrade{*latestOlder}, window.ShortTrades...)
	}
}

func appendLegacyTrade(window *legacyWindow, isBuy bool, vol float64, price float64, ts int64) {
	window.LastPrice = price
	window.LastUnixTs = ts
	if window.FirstPrice == 0 {
		window.FirstPrice = price
	}

	bucketIdx := (ts / 60) % 1440
	if bucketIdx < 0 {
		bucketIdx += 1440
	}
	bucketStartTs := (ts / 60) * 60

	mb := &window.MinuteBuckets[bucketIdx]
	if mb.BucketStart != bucketStartTs {
		*mb = MinuteBucket{
			BucketStart: bucketStartTs,
			OpenPrice:   price,
			ClosePrice:  price,
		}
	} else if mb.OpenPrice == 0 {
		mb.OpenPrice = price
	}
	mb.ClosePrice = price
	if isBuy {
		mb.Buys++
		mb.BuyVolume += vol
	} else {
		mb.Sells++
		mb.SellVolume += vol
	}
	mb.Volume += vol
}

func summarizeLegacyShortTrades(trades []ShortTrade, nowTs int64, windowSeconds int64, lastPrice float64, firstPrice float64) (buys int, sells int, buyVolume float64, sellVolume float64, volume float64, anchorPrice float64) {
	cutoff := nowTs - windowSeconds
	anchorPrice = lastPrice

	var latestBefore *ShortTrade
	var earliestInside *ShortTrade
	for _, trade := range trades {
		if trade.Timestamp < cutoff {
			if latestBefore == nil || trade.Timestamp > latestBefore.Timestamp {
				copyTrade := trade
				latestBefore = &copyTrade
			}
			continue
		}

		volume += trade.Volume
		if trade.IsBuy {
			buys++
			buyVolume += trade.Volume
		} else {
			sells++
			sellVolume += trade.Volume
		}
		if earliestInside == nil || trade.Timestamp < earliestInside.Timestamp {
			copyTrade := trade
			earliestInside = &copyTrade
		}
	}

	switch {
	case latestBefore != nil:
		anchorPrice = latestBefore.Price
	case earliestInside != nil:
		anchorPrice = earliestInside.Price
	case firstPrice > 0:
		anchorPrice = firstPrice
	}

	return
}

func summarizeLegacyMinuteBuckets(window *legacyWindow, nowTs int64, minsBack int64) (buys int, sells int, buyVolume float64, sellVolume float64, volume float64, anchorPrice float64) {
	anchorPrice = window.LastPrice
	currentBucketStart := (nowTs / 60) * 60

	var earliestOpen float64
	var earliestOpenTs int64

	for offset := int64(0); offset < minsBack; offset++ {
		bucketStart := currentBucketStart - offset*60
		idx := (bucketStart / 60) % 1440
		if idx < 0 {
			idx += 1440
		}
		bucket := window.MinuteBuckets[idx]
		if bucket.BucketStart != bucketStart || bucket.ClosePrice == 0 {
			continue
		}

		buys += bucket.Buys
		sells += bucket.Sells
		volume += bucket.Volume
		buyVolume += bucket.BuyVolume
		sellVolume += bucket.SellVolume
		if earliestOpenTs == 0 || bucketStart < earliestOpenTs {
			earliestOpenTs = bucketStart
			earliestOpen = bucket.OpenPrice
		}
	}

	for offset := minsBack; offset < 1440; offset++ {
		bucketStart := currentBucketStart - offset*60
		idx := (bucketStart / 60) % 1440
		if idx < 0 {
			idx += 1440
		}
		bucket := window.MinuteBuckets[idx]
		if bucket.BucketStart == bucketStart && bucket.ClosePrice > 0 {
			anchorPrice = bucket.ClosePrice
			return
		}
	}

	if earliestOpenTs != 0 {
		anchorPrice = earliestOpen
	} else if window.FirstPrice > 0 {
		anchorPrice = window.FirstPrice
	}
	return
}

func generateLegacyPayload(mint string, window *legacyWindow, nowTs int64) TokenStatsPayload {
	payload := TokenStatsPayload{
		Mint:      mint,
		Price:     window.LastPrice,
		Timestamp: window.LastUnixTs,
		P1m:       window.LastPrice,
		P5m:       window.LastPrice,
		P1h:       window.LastPrice,
		P4h:       window.LastPrice,
		P24h:      window.LastPrice,
	}

	payload.B1m, payload.S1m, payload.BV1m, payload.SV1m, payload.V1m, payload.P1m =
		summarizeLegacyShortTrades(window.ShortTrades, nowTs, 60, window.LastPrice, window.FirstPrice)
	payload.B5m, payload.S5m, payload.BV5m, payload.SV5m, payload.V5m, payload.P5m =
		summarizeLegacyShortTrades(window.ShortTrades, nowTs, shortWindowSeconds, window.LastPrice, window.FirstPrice)
	payload.B1h, payload.S1h, payload.BV1h, payload.SV1h, payload.V1h, payload.P1h =
		summarizeLegacyMinuteBuckets(window, nowTs, 60)
	payload.B4h, payload.S4h, payload.BV4h, payload.SV4h, payload.V4h, payload.P4h =
		summarizeLegacyMinuteBuckets(window, nowTs, 240)
	payload.B24h, payload.S24h, payload.BV24h, payload.SV24h, payload.V24h, payload.P24h =
		summarizeLegacyMinuteBuckets(window, nowTs, 1440)
	return payload
}
