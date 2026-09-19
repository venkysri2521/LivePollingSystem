package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/venkat/livepolls/backend/internal/realtime"
)

// Stream is the live results channel, served as Server-Sent Events.
//
// SSE over WebSockets here because the traffic is one-directional - the server
// pushes counts, the client never pushes back over the same connection - and
// because EventSource reconnects on its own and rides through proxies and free
// hosting tiers that drop idle WebSocket upgrades.
//
// Each connection subscribes to that poll's Redis channel, so a vote received
// by any instance reaches viewers attached to every other instance.
func (h *PollHandler) Stream(c *gin.Context) {
	ctx := c.Request.Context()

	poll, err := h.load(ctx, c.Param("slug"))
	if err != nil {
		notFoundOrFail(c, err)
		return
	}

	flusher, ok := c.Writer.(http.Flusher)
	if !ok {
		fail(c, http.StatusInternalServerError, "streaming is not supported here")
		return
	}

	c.Writer.Header().Set("Content-Type", "text/event-stream")
	c.Writer.Header().Set("Cache-Control", "no-cache, no-transform")
	c.Writer.Header().Set("Connection", "keep-alive")
	c.Writer.Header().Set("X-Accel-Buffering", "no") // stops nginx buffering the stream
	c.Writer.WriteHeader(http.StatusOK)

	pubsub := h.RT.Subscribe(context.Background(), poll.Slug)
	defer pubsub.Close()
	messages := pubsub.Channel()

	viewers := h.RT.Viewers(ctx, poll.Slug, 1)
	defer func() {
		// Background context: the request context is already cancelled here.
		h.RT.Viewers(context.Background(), poll.Slug, -1)
	}()

	// Send the current state immediately so a late joiner is never staring at
	// an empty board waiting for somebody else to vote.
	counts, total, err := h.RT.Counts(ctx, poll)
	if err == nil {
		writeEvent(c, flusher, realtime.Snapshot{
			Type: "results", Slug: poll.Slug, Counts: counts, Total: total,
			Viewers: viewers, At: time.Now().UnixMilli(),
		})
	}

	// Comment pings keep proxies and load balancers from reaping an idle
	// connection during a quiet stretch of a poll.
	ping := time.NewTicker(20 * time.Second)
	defer ping.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case msg, open := <-messages:
			if !open {
				return
			}
			fmt.Fprintf(c.Writer, "data: %s\n\n", msg.Payload)
			flusher.Flush()

		case <-ping.C:
			fmt.Fprint(c.Writer, ": ping\n\n")
			flusher.Flush()
		}
	}
}

func writeEvent(c *gin.Context, flusher http.Flusher, snap realtime.Snapshot) {
	b, err := json.Marshal(snap)
	if err != nil {
		return
	}
	fmt.Fprintf(c.Writer, "retry: 3000\ndata: %s\n\n", b)
	flusher.Flush()
}
