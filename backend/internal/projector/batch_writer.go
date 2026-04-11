package projector

import (
	"context"

	"solana-dashboard-go/internal/store"
)

type batchCapableWriter interface {
	readModelWriter
	InsertTradeEventsBatch(ctx context.Context, trades []store.TradeEventRecord) error
	InsertActivityEventsBatch(ctx context.Context, activities []store.ActivityEventRecord) error
}

type queuedStateOp struct {
	token       *store.TokenRecord
	metadata    *store.TokenMetadataRecord
	market      *store.TokenMarketRecord
	closeMarket *closeMarketOp
}

type closeMarketOp struct {
	marketID string
	endedAt  int64
}

type writeBatch struct {
	store      batchCapableWriter
	stateOps   []queuedStateOp
	trades     []store.TradeEventRecord
	activities []store.ActivityEventRecord
}

func newWriteBatch(store batchCapableWriter) *writeBatch {
	return &writeBatch{store: store}
}

func (w *writeBatch) UpsertToken(ctx context.Context, record store.TokenRecord) error {
	copy := record
	w.stateOps = append(w.stateOps, queuedStateOp{token: &copy})
	return nil
}

func (w *writeBatch) UpsertTokenMetadata(ctx context.Context, record store.TokenMetadataRecord) error {
	copy := record
	w.stateOps = append(w.stateOps, queuedStateOp{metadata: &copy})
	return nil
}

func (w *writeBatch) UpsertTokenMarket(ctx context.Context, record store.TokenMarketRecord) error {
	copy := record
	w.stateOps = append(w.stateOps, queuedStateOp{market: &copy})
	return nil
}

func (w *writeBatch) CloseTokenMarket(ctx context.Context, marketID string, endedAt int64) error {
	w.stateOps = append(w.stateOps, queuedStateOp{
		closeMarket: &closeMarketOp{marketID: marketID, endedAt: endedAt},
	})
	return nil
}

func (w *writeBatch) InsertTradeEvent(ctx context.Context, trade store.TradeEventRecord) error {
	w.trades = append(w.trades, trade)
	return nil
}

func (w *writeBatch) InsertActivityEvent(ctx context.Context, activity store.ActivityEventRecord) error {
	w.activities = append(w.activities, activity)
	return nil
}

func (w *writeBatch) Flush(ctx context.Context) error {
	for _, op := range w.stateOps {
		switch {
		case op.token != nil:
			if err := w.store.UpsertToken(ctx, *op.token); err != nil {
				return err
			}
		case op.metadata != nil:
			if err := w.store.UpsertTokenMetadata(ctx, *op.metadata); err != nil {
				return err
			}
		case op.market != nil:
			if err := w.store.UpsertTokenMarket(ctx, *op.market); err != nil {
				return err
			}
		case op.closeMarket != nil:
			if err := w.store.CloseTokenMarket(ctx, op.closeMarket.marketID, op.closeMarket.endedAt); err != nil {
				return err
			}
		}
	}

	if err := w.store.InsertTradeEventsBatch(ctx, w.trades); err != nil {
		return err
	}
	if err := w.store.InsertActivityEventsBatch(ctx, w.activities); err != nil {
		return err
	}
	return nil
}
