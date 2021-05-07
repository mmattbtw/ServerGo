package api_websocket

import (
	"context"

	"github.com/SevenTV/ServerGo/src/mongo"
	"github.com/gofiber/websocket/v2"
	"github.com/google/uuid"
)

func createEmoteSubscription(ctx context.Context) {
	id, err := uuid.NewUUID()
	if err != nil {
		sendClosure(ctx, websocket.CloseInternalServerErr, "")
	}

	// Subscribe to mongo changestream for users
	ch := make(chan mongo.ChangeStreamEvent)
	mongo.Subscribe("users", id, ch)

	for {
		select {
		case ev := <-ch: // Listen for changes
			// Filter to update operations
			if ev.OperationType != "update" {
				continue
			}

			// Increase sequence
			seq := ctx.Value("seq").(int32)
			seq++
			ctx = context.WithValue(ctx, "seq", seq)

			// Send dispatch
			sendOpDispatch(ctx, ev.FullDocument, seq)
		case <-ctx.Done():
			mongo.Unsubscribe(id)
			return
		}
	}
}
