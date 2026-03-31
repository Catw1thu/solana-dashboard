package httpapi

import (
	"context"
	"net/http"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
)

func (h *Handler) ServeWS(w http.ResponseWriter, r *http.Request) {
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		OriginPatterns: []string{"localhost:*", "127.0.0.1:*"},
	})
	if err != nil {
		return
	}
	defer conn.CloseNow()

	ctx := context.Background()
	readCtx := conn.CloseRead(ctx)

	ch := h.service.Subscribe(64)
	defer h.service.Unsubscribe(ch)

	for {
		select {
		case <-readCtx.Done():
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			if err := wsjson.Write(ctx, conn, event); err != nil {
				return
			}
		}
	}

}
