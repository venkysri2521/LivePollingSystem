package handlers

import (
	"context"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"

	"github.com/venkat/livepolls/backend/internal/models"
	"github.com/venkat/livepolls/backend/internal/realtime"
)

type voteReq struct {
	OptionIDs []string `json:"optionIds"`
}

// Vote is the hot path, and the ordering below is deliberate:
//
//	validate -> claim the voter in Redis -> increment in Redis -> publish ->
//	respond -> persist to Mongo in the background
//
// The voter claim happens before anything is counted, so a double-click or a
// replayed request cannot land twice. Mongo is written after the response
// because the durable record is not what the audience is waiting on; if that
// write fails the counts are rebuilt from Redis on the next read, and the
// unique index still refuses a genuine duplicate.
func (h *PollHandler) Vote(c *gin.Context) {
	var req voteReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "send a valid JSON body")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	poll, err := h.load(ctx, c.Param("slug"))
	if err != nil {
		notFoundOrFail(c, err)
		return
	}
	if !poll.IsAcceptingVotes(time.Now().UTC()) {
		fail(c, http.StatusConflict, "this poll is closed")
		return
	}

	// Every submitted id must belong to this poll. Without this check a client
	// could invent an option id and write a stray field into the counts hash.
	chosen, err := cleanChoices(poll, req.OptionIDs)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}

	voterKey := h.voterKey(c, poll.Slug)
	claimed, err := h.RT.ClaimVoter(ctx, poll.Slug, voterKey)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not record your vote")
		return
	}
	if !claimed {
		fail(c, http.StatusConflict, "you have already voted in this poll")
		return
	}

	counts, total, err := h.RT.Increment(ctx, poll, chosen)
	if err != nil {
		h.RT.ReleaseVoter(ctx, poll.Slug, voterKey)
		fail(c, http.StatusInternalServerError, "could not record your vote")
		return
	}

	snap := realtime.Snapshot{
		Type:   "results",
		Slug:   poll.Slug,
		Counts: counts,
		Total:  total,
		At:     time.Now().UnixMilli(),
	}
	h.RT.Publish(ctx, poll.Slug, snap)

	c.JSON(http.StatusOK, gin.H{"counts": counts, "total": total, "hasVoted": true})

	// Detached context: the durable write must survive the request returning.
	go h.persistVote(poll, chosen, voterKey)
}

func (h *PollHandler) persistVote(poll *models.Poll, chosen []string, voterKey string) {
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	_, err := h.Mongo.Votes().InsertOne(ctx, models.Vote{
		PollID:    poll.ID,
		OptionIDs: chosen,
		VoterKey:  voterKey,
		CreatedAt: time.Now().UTC(),
	})
	if err != nil {
		if mongo.IsDuplicateKeyError(err) {
			return // already durable, nothing to do
		}
		log.Printf("vote: durable write failed for poll %s: %v", poll.Slug, err)
		return
	}

	inc := bson.M{"totalVotes": 1}
	for _, id := range chosen {
		inc["counts."+id] = 1
	}
	if _, err := h.Mongo.Polls().UpdateByID(ctx, poll.ID, bson.M{"$inc": inc}); err != nil {
		log.Printf("vote: count rollup failed for poll %s: %v", poll.Slug, err)
	}
}

func cleanChoices(poll *models.Poll, raw []string) ([]string, error) {
	if len(raw) == 0 {
		return nil, errBadChoice("pick an option before submitting")
	}
	if len(raw) > len(poll.Options) {
		return nil, errBadChoice("too many options selected")
	}
	if !poll.AllowMultiple && len(raw) > 1 {
		return nil, errBadChoice("this poll accepts one answer")
	}

	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, id := range raw {
		if !poll.HasOption(id) {
			return nil, errBadChoice("that option is not part of this poll")
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out, nil
}

type errBadChoice string

func (e errBadChoice) Error() string { return string(e) }
