package handlers

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"

	"github.com/venkat/livepolls/backend/internal/config"
	"github.com/venkat/livepolls/backend/internal/db"
	"github.com/venkat/livepolls/backend/internal/middleware"
	"github.com/venkat/livepolls/backend/internal/models"
	"github.com/venkat/livepolls/backend/internal/realtime"
	"github.com/venkat/livepolls/backend/internal/validate"
)

type PollHandler struct {
	Mongo *db.Mongo
	RT    *realtime.Service
	Cfg   *config.Config
}

const voterCookie = "lp_voter"

type createPollReq struct {
	Question      string   `json:"question"`
	Options       []string `json:"options"`
	AllowMultiple bool     `json:"allowMultiple"`
	HideUntilVote bool     `json:"hideUntilVote"`
	CloseInHours  int      `json:"closeInHours"`
}

// Create validates everything server-side, then writes the poll to Mongo and
// seeds the Redis counter hash so the first viewer gets a warm read.
func (h *PollHandler) Create(c *gin.Context) {
	var req createPollReq
	if err := c.ShouldBindJSON(&req); err != nil {
		fail(c, http.StatusBadRequest, "send a valid JSON body")
		return
	}

	question, err := validate.Question(req.Question)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	optionTexts, err := validate.Options(req.Options)
	if err != nil {
		fail(c, http.StatusBadRequest, err.Error())
		return
	}
	if req.CloseInHours < 0 || req.CloseInHours > 24*30 {
		fail(c, http.StatusBadRequest, "closing time must be between 0 and 720 hours")
		return
	}

	ownerID, err := primitive.ObjectIDFromHex(middleware.UserID(c))
	if err != nil {
		fail(c, http.StatusUnauthorized, "sign in to continue")
		return
	}

	pollOptions := make([]models.Option, len(optionTexts))
	counts := make(map[string]int64, len(optionTexts))
	for i, text := range optionTexts {
		// Short positional ids keep the Mongo counts map dot-free, which
		// matters because "counts.<id>" is used directly in $inc paths.
		id := "o" + strconv.Itoa(i+1)
		pollOptions[i] = models.Option{ID: id, Text: text}
		counts[id] = 0
	}

	poll := models.Poll{
		Slug:          newSlug(),
		Question:      question,
		Options:       pollOptions,
		OwnerID:       ownerID,
		OwnerName:     middleware.UserName(c),
		Status:        models.StatusOpen,
		AllowMultiple: req.AllowMultiple,
		HideUntilVote: req.HideUntilVote,
		Counts:        counts,
		TotalVotes:    0,
		CreatedAt:     time.Now().UTC(),
	}
	if req.CloseInHours > 0 {
		t := poll.CreatedAt.Add(time.Duration(req.CloseInHours) * time.Hour)
		poll.ClosesAt = &t
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	res, err := h.Mongo.Polls().InsertOne(ctx, poll)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not create the poll")
		return
	}
	poll.ID = res.InsertedID.(primitive.ObjectID)

	_ = h.RT.Rehydrate(ctx, &poll)
	h.RT.CachePoll(ctx, &poll)

	c.JSON(http.StatusCreated, gin.H{
		"poll":     poll,
		"shareUrl": h.shareURL(poll.Slug),
		"hasVoted": false,
		"isOwner":  true,
	})
}

// Get serves the poll page. Reads come from the Redis cache first; Mongo is
// only touched on a cache miss.
func (h *PollHandler) Get(c *gin.Context) {
	slug := strings.TrimSpace(c.Param("slug"))
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	poll, err := h.load(ctx, slug)
	if err != nil {
		notFoundOrFail(c, err)
		return
	}

	counts, total, err := h.RT.Counts(ctx, poll)
	if err != nil {
		counts, total = poll.Counts, poll.TotalVotes
	}

	voterKey := h.voterKey(c, poll.Slug)
	c.JSON(http.StatusOK, gin.H{
		"poll":     poll,
		"counts":   counts,
		"total":    total,
		"hasVoted": h.RT.HasVoted(ctx, poll.Slug, voterKey),
		"isOwner":  middleware.UserID(c) == poll.OwnerID.Hex(),
		"shareUrl": h.shareURL(poll.Slug),
		"open":     poll.IsAcceptingVotes(time.Now().UTC()),
	})
}

// Results is the polling fallback for clients that cannot hold an SSE stream.
func (h *PollHandler) Results(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	poll, err := h.load(ctx, c.Param("slug"))
	if err != nil {
		notFoundOrFail(c, err)
		return
	}
	counts, total, err := h.RT.Counts(ctx, poll)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not read the results")
		return
	}
	c.JSON(http.StatusOK, realtime.Snapshot{
		Type: "results", Slug: poll.Slug, Counts: counts, Total: total, At: time.Now().UnixMilli(),
	})
}

// Mine lists the signed-in user's polls, newest first.
func (h *PollHandler) Mine(c *gin.Context) {
	ownerID, err := primitive.ObjectIDFromHex(middleware.UserID(c))
	if err != nil {
		fail(c, http.StatusUnauthorized, "sign in to continue")
		return
	}
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	cur, err := h.Mongo.Polls().Find(ctx,
		bson.M{"ownerId": ownerID},
		options.Find().SetSort(bson.D{{Key: "createdAt", Value: -1}}).SetLimit(100),
	)
	if err != nil {
		fail(c, http.StatusInternalServerError, "could not load your polls")
		return
	}
	defer cur.Close(ctx)

	polls := make([]models.Poll, 0)
	if err := cur.All(ctx, &polls); err != nil {
		fail(c, http.StatusInternalServerError, "could not load your polls")
		return
	}

	// Redis holds the live totals, so overlay them on the stored documents.
	for i := range polls {
		if counts, total, err := h.RT.Counts(ctx, &polls[i]); err == nil {
			polls[i].Counts = counts
			polls[i].TotalVotes = total
		}
	}
	c.JSON(http.StatusOK, gin.H{"polls": polls})
}

// SetStatus lets the owner close or reopen a poll. Closing publishes an event
// so every open tab switches to the final state without a refresh.
func (h *PollHandler) SetStatus(c *gin.Context) {
	var req struct {
		Status string `json:"status"`
	}
	if err := c.ShouldBindJSON(&req); err != nil ||
		(req.Status != models.StatusOpen && req.Status != models.StatusClosed) {
		fail(c, http.StatusBadRequest, "status must be open or closed")
		return
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	poll, err := h.loadOwned(ctx, c)
	if err != nil {
		notFoundOrFail(c, err)
		return
	}

	if _, err := h.Mongo.Polls().UpdateByID(ctx, poll.ID,
		bson.M{"$set": bson.M{"status": req.Status}}); err != nil {
		fail(c, http.StatusInternalServerError, "could not update the poll")
		return
	}
	poll.Status = req.Status
	h.RT.InvalidatePoll(ctx, poll.Slug)

	counts, total, _ := h.RT.Counts(ctx, poll)
	kind := "results"
	if req.Status == models.StatusClosed {
		kind = "closed"
	}
	h.RT.Publish(ctx, poll.Slug, realtime.Snapshot{
		Type: kind, Slug: poll.Slug, Counts: counts, Total: total, At: time.Now().UnixMilli(),
	})

	c.JSON(http.StatusOK, gin.H{"poll": poll})
}

func (h *PollHandler) Delete(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 10*time.Second)
	defer cancel()

	poll, err := h.loadOwned(ctx, c)
	if err != nil {
		notFoundOrFail(c, err)
		return
	}
	if _, err := h.Mongo.Polls().DeleteOne(ctx, bson.M{"_id": poll.ID}); err != nil {
		fail(c, http.StatusInternalServerError, "could not delete the poll")
		return
	}
	_, _ = h.Mongo.Votes().DeleteMany(ctx, bson.M{"pollId": poll.ID})
	h.RT.InvalidatePoll(ctx, poll.Slug)

	c.JSON(http.StatusOK, gin.H{"deleted": true})
}

// ---------- shared helpers ----------

var errNotFound = errors.New("poll not found")
var errForbidden = errors.New("not your poll")

func (h *PollHandler) load(ctx context.Context, slug string) (*models.Poll, error) {
	slug = validate.Clean(slug)
	if len(slug) < 4 || len(slug) > 24 {
		return nil, errNotFound
	}
	if p, ok := h.RT.CachedPoll(ctx, slug); ok {
		return p, nil
	}
	var p models.Poll
	if err := h.Mongo.Polls().FindOne(ctx, bson.M{"slug": slug}).Decode(&p); err != nil {
		if errors.Is(err, mongo.ErrNoDocuments) {
			return nil, errNotFound
		}
		return nil, err
	}
	h.RT.CachePoll(ctx, &p)
	return &p, nil
}

func (h *PollHandler) loadOwned(ctx context.Context, c *gin.Context) (*models.Poll, error) {
	poll, err := h.load(ctx, c.Param("slug"))
	if err != nil {
		return nil, err
	}
	if poll.OwnerID.Hex() != middleware.UserID(c) {
		return nil, errForbidden
	}
	return poll, nil
}

func notFoundOrFail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, errNotFound):
		fail(c, http.StatusNotFound, "that poll does not exist")
	case errors.Is(err, errForbidden):
		fail(c, http.StatusForbidden, "you can only manage polls you created")
	default:
		fail(c, http.StatusInternalServerError, "something went wrong")
	}
}

func (h *PollHandler) shareURL(slug string) string {
	return strings.TrimRight(h.Cfg.PublicAppURL, "/") + "/p/" + slug
}

// voterKey identifies a voter without storing anything identifying. A signed-in
// user is keyed by account; anyone else gets a random cookie. Either way the
// value is hashed with the poll slug before storage, so the same visitor is not
// traceable across polls.
func (h *PollHandler) voterKey(c *gin.Context, slug string) string {
	if uid := middleware.UserID(c); uid != "" {
		return hashKey("u:" + uid + ":" + slug)
	}
	raw, err := c.Cookie(voterCookie)
	if err != nil || len(raw) < 16 || len(raw) > 64 {
		raw = randomToken(24)
		c.SetSameSite(http.SameSiteLaxMode)
		c.SetCookie(voterCookie, raw, int((180 * 24 * time.Hour).Seconds()), "/", "", h.Cfg.IsProd(), true)
	}
	return hashKey("a:" + raw + ":" + slug)
}

func hashKey(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:16])
}

func randomToken(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return time.Now().Format("20060102150405.000000000")
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

const slugAlphabet = "abcdefghijkmnpqrstuvwxyz23456789" // no look-alike characters

func newSlug() string {
	b := make([]byte, 8)
	max := big.NewInt(int64(len(slugAlphabet)))
	for i := range b {
		n, err := rand.Int(rand.Reader, max)
		if err != nil {
			b[i] = slugAlphabet[i%len(slugAlphabet)]
			continue
		}
		b[i] = slugAlphabet[n.Int64()]
	}
	return string(b)
}
