package broadcaster

import (
	"context"
	"encoding/json"
	"log"
	"strconv"
	"sync"
	"time"

	"solana-dashboard-go/internal/events"
	"solana-dashboard-go/internal/realtime"
	"solana-dashboard-go/internal/store"

	"github.com/nats-io/nats.go"
)

const (
	wrappedSolMint       = "So11111111111111111111111111111111111111112"
	solDecimals          = 9
	defaultTokenDecimals = 6
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
	Volume    float64 // Quote Volume (SOL)
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

type TokenSlidingWindow struct {
	mu            sync.Mutex
	Seeded        bool
	LastPrice     float64
	LastUnixTs    int64
	FirstPrice    float64
	FirstUnixTs   int64
	ShortTrades   []ShortTrade
	MinuteBuckets [1440]MinuteBucket
}

type Broadcaster struct {
	nc           *nats.Conn
	hub          *realtime.Hub
	metricsStore tradeMetricsStore
	windows      map[string]*TokenSlidingWindow
	mu           sync.RWMutex
}

type tradeMetricsStore interface {
	ListTradeMetricsForStatsByMint(ctx context.Context, mint string) ([]store.TradeMetricPoint, error)
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
		nc:           nc,
		hub:          hub,
		metricsStore: metricsStore,
		windows:      make(map[string]*TokenSlidingWindow),
	}
}

func (b *Broadcaster) getWindow(mint string) *TokenSlidingWindow {
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
		w = &TokenSlidingWindow{}
		b.windows[mint] = w
	}
	return w
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

	b.ensureWindowSeeded(payload.Mint, time.Now().Unix())
	b.recordTrade(payload.Mint, payload.Side == "buy", vol, price, env.EventUnixTS)
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

	b.ensureWindowSeeded(mint, time.Now().Unix())
	b.recordTrade(mint, isBuy, vol, price, env.EventUnixTS)
}

func (b *Broadcaster) recordTrade(mint string, isBuy bool, vol float64, price float64, ts int64) {
	if mint == "" {
		return
	}

	w := b.getWindow(mint)
	w.mu.Lock()
	defer w.mu.Unlock()

	appendTradeMetrics(w, isBuy, vol, price, ts)
	w.ShortTrades = append(w.ShortTrades, ShortTrade{
		Timestamp: ts,
		IsBuy:     isBuy,
		Volume:    vol,
		Price:     price,
	})
}

func appendTradeMetrics(w *TokenSlidingWindow, isBuy bool, vol float64, price float64, ts int64) {
	w.LastPrice = price
	w.LastUnixTs = ts
	if w.FirstUnixTs == 0 || ts < w.FirstUnixTs {
		w.FirstUnixTs = ts
		w.FirstPrice = price
	}

	bucketIdx := (ts / 60) % 1440
	bucketStartTs := (ts / 60) * 60

	mb := &w.MinuteBuckets[bucketIdx]
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

func appendShortTrade(trades []ShortTrade, trade ShortTrade) []ShortTrade {
	return append(trades, trade)
}

func seedWindowFromTradeMetrics(w *TokenSlidingWindow, metrics []store.TradeMetricPoint, nowTs int64) {
	if len(metrics) == 0 {
		return
	}

	cutoff5m := nowTs - 300
	var latestOlder *ShortTrade

	for _, metric := range metrics {
		isBuy := metric.Side == "buy"
		appendTradeMetrics(w, isBuy, metric.Volume, metric.Price, metric.EventUnixTS)

		shortTrade := ShortTrade{
			Timestamp: metric.EventUnixTS,
			IsBuy:     isBuy,
			Volume:    metric.Volume,
			Price:     metric.Price,
		}
		if metric.EventUnixTS >= cutoff5m {
			w.ShortTrades = appendShortTrade(w.ShortTrades, shortTrade)
			continue
		}

		copy := shortTrade
		latestOlder = &copy
	}

	if latestOlder != nil {
		w.ShortTrades = append([]ShortTrade{*latestOlder}, w.ShortTrades...)
	}
}

func BuildPayloadFromTradeMetrics(mint string, metrics []store.TradeMetricPoint, nowTs int64) (TokenStatsPayload, bool) {
	if len(metrics) == 0 {
		return TokenStatsPayload{}, false
	}

	window := &TokenSlidingWindow{}
	seedWindowFromTradeMetrics(window, metrics, nowTs)
	if window.LastPrice <= 0 {
		return TokenStatsPayload{}, false
	}

	return generatePayloadForWindow(mint, window, nowTs), true
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

	metrics, err := b.metricsStore.ListTradeMetricsForStatsByMint(context.Background(), mint)
	if err != nil {
		log.Printf("[stats] failed to seed window for mint=%s: %v", mint, err)
		return
	}

	window.mu.Lock()
	defer window.mu.Unlock()
	if window.Seeded {
		return
	}

	seedWindowFromTradeMetrics(window, metrics, nowTs)
	window.Seeded = true
}

func isPumpfunTradeEvent(env *events.Envelope) bool {
	if env == nil {
		return false
	}

	if env.Protocol == "pumpfun" && env.EventType == "trade" {
		return true
	}

	// Backward compatibility for legacy flattened event_type values.
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

func summarizeShortTrades(trades []ShortTrade, nowTs int64, windowSeconds int64, lastPrice float64, firstPrice float64) (buys int, sells int, buyVolume float64, sellVolume float64, volume float64, anchorPrice float64) {
	cutoff := nowTs - windowSeconds
	anchorPrice = lastPrice

	var latestBefore *ShortTrade
	var earliestInside *ShortTrade

	for _, trade := range trades {
		if trade.Timestamp < cutoff {
			if latestBefore == nil || trade.Timestamp > latestBefore.Timestamp {
				copy := trade
				latestBefore = &copy
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
			copy := trade
			earliestInside = &copy
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

func summarizeMinuteBuckets(w *TokenSlidingWindow, nowTs int64, minsBack int64) (buys int, sells int, buyVolume float64, sellVolume float64, volume float64, anchorPrice float64) {
	anchorPrice = w.LastPrice
	currentBucketStart := (nowTs / 60) * 60

	var earliestOpen float64
	var earliestOpenTs int64

	for offset := int64(0); offset < minsBack; offset++ {
		bucketStart := currentBucketStart - offset*60
		idx := (bucketStart / 60) % 1440
		if idx < 0 {
			idx += 1440
		}

		bucket := w.MinuteBuckets[idx]
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

		bucket := w.MinuteBuckets[idx]
		if bucket.BucketStart == bucketStart && bucket.ClosePrice > 0 {
			anchorPrice = bucket.ClosePrice
			return
		}
	}

	switch {
	case earliestOpenTs != 0:
		anchorPrice = earliestOpen
	case w.FirstPrice > 0:
		anchorPrice = w.FirstPrice
	}

	return
}

func generatePayloadForWindow(mint string, w *TokenSlidingWindow, nowTs int64) TokenStatsPayload {
	payload := TokenStatsPayload{
		Mint:      mint,
		Price:     w.LastPrice,
		Timestamp: w.LastUnixTs,
		P1m:       w.LastPrice,
		P5m:       w.LastPrice,
		P1h:       w.LastPrice,
		P4h:       w.LastPrice,
		P24h:      w.LastPrice,
	}

	payload.B1m, payload.S1m, payload.BV1m, payload.SV1m, payload.V1m, payload.P1m =
		summarizeShortTrades(w.ShortTrades, nowTs, 60, w.LastPrice, w.FirstPrice)
	payload.B5m, payload.S5m, payload.BV5m, payload.SV5m, payload.V5m, payload.P5m =
		summarizeShortTrades(w.ShortTrades, nowTs, 300, w.LastPrice, w.FirstPrice)
	payload.B1h, payload.S1h, payload.BV1h, payload.SV1h, payload.V1h, payload.P1h =
		summarizeMinuteBuckets(w, nowTs, 60)
	payload.B4h, payload.S4h, payload.BV4h, payload.SV4h, payload.V4h, payload.P4h =
		summarizeMinuteBuckets(w, nowTs, 240)
	payload.B24h, payload.S24h, payload.BV24h, payload.SV24h, payload.V24h, payload.P24h =
		summarizeMinuteBuckets(w, nowTs, 1440)

	return payload
}

func (b *Broadcaster) generatePayload(mint string, w *TokenSlidingWindow, nowTs int64) TokenStatsPayload {
	return generatePayloadForWindow(mint, w, nowTs)
}

func (b *Broadcaster) Sweep() {
	nowTs := time.Now().Unix()
	cutoff5m := nowTs - 300

	b.mu.RLock()
	var snapshot []*TokenSlidingWindow
	var mints []string
	for m, w := range b.windows {
		snapshot = append(snapshot, w)
		mints = append(mints, m)
	}
	b.mu.RUnlock()

	for idx, w := range snapshot {
		w.mu.Lock()

		// Prune short trades
		var active []ShortTrade
		var latestOlder *ShortTrade
		for _, st := range w.ShortTrades {
			if st.Timestamp >= cutoff5m {
				active = append(active, st)
				continue
			}

			if latestOlder == nil || st.Timestamp > latestOlder.Timestamp {
				copy := st
				latestOlder = &copy
			}
		}
		if latestOlder != nil {
			active = append([]ShortTrade{*latestOlder}, active...)
		}
		w.ShortTrades = active

		payload := b.generatePayload(mints[idx], w, nowTs)
		w.mu.Unlock()

		// Publish to WS
		bs, _ := json.Marshal(payload)
		b.hub.Publish("token:"+mints[idx], events.Envelope{
			EventType:   "token_stat",
			Payload:     json.RawMessage(bs),
			EventUnixTS: nowTs,
		})
	}
}

func (b *Broadcaster) Run(ctx context.Context) error {
	var hubEvents chan events.Envelope
	if b.hub != nil {
		hubEvents = b.hub.Subscribe("global", 1024)
		defer b.hub.Unsubscribe(hubEvents)
	}

	if hubEvents == nil && b.nc != nil {
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

	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil
		case event, ok := <-hubEvents:
			if !ok {
				hubEvents = nil
				continue
			}
			b.handleTrade(&event)
		case <-ticker.C:
			b.Sweep()
		}
	}
}
