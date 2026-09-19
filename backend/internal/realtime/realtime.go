// Package realtime is the Redis layer. Redis owns four jobs here:
//
//  1. Live counters   - a hash per poll, incremented atomically on every vote.
//     This is what the UI reads, not Mongo.
//  2. Fan-out         - pub/sub carries each vote to every connected SSE
//     stream, including streams held by other server instances.
//  3. Duplicate guard - a set of voter keys per poll, claimed with SADD so the
//     check and the write are one atomic operation.
//  4. Read cache      - poll metadata is cached so the vote path and every new
//     viewer avoid a Mongo round trip.
//
// Mongo remains the source of truth. If Redis is wiped, Rehydrate rebuilds the
// counters and the voter set from the votes collection.
package realtime

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/venkat/livepolls/backend/internal/models"
)

const keyTTL = 30 * 24 * time.Hour

type Service struct {
	rdb *redis.Client
}

func New(rdb *redis.Client) *Service { return &Service{rdb: rdb} }

func countsKey(slug string) string { return "poll:" + slug + ":counts" }
func votersKey(slug string) string { return "poll:" + slug + ":voters" }
func metaKey(slug string) string   { return "poll:" + slug + ":meta" }
func channel(slug string) string   { return "poll:" + slug + ":events" }

// Snapshot is the single shape the client ever receives for results, whether
// it arrives over SSE or from the plain REST fallback.
type Snapshot struct {
	Type    string           `json:"type"` // results | closed
	Slug    string           `json:"slug"`
	Counts  map[string]int64 `json:"counts"`
	Total   int64            `json:"total"`
	Viewers int64            `json:"viewers,omitempty"`
	At      int64            `json:"at"`
}

// ---------- poll metadata cache ----------

func (s *Service) CachePoll(ctx context.Context, p *models.Poll) {
	b, err := json.Marshal(p)
	if err != nil {
		return
	}
	s.rdb.Set(ctx, metaKey(p.Slug), b, 10*time.Minute)
}

func (s *Service) CachedPoll(ctx context.Context, slug string) (*models.Poll, bool) {
	b, err := s.rdb.Get(ctx, metaKey(slug)).Bytes()
	if err != nil {
		return nil, false
	}
	var p models.Poll
	if err := json.Unmarshal(b, &p); err != nil {
		return nil, false
	}
	return &p, true
}

func (s *Service) InvalidatePoll(ctx context.Context, slug string) {
	s.rdb.Del(ctx, metaKey(slug))
}

// ---------- counters ----------

// Rehydrate seeds the counter hash from the values Mongo already holds, but
// only if the hash is missing. SETNX-style semantics keep a concurrent request
// from resetting counts that are already live.
func (s *Service) Rehydrate(ctx context.Context, p *models.Poll) error {
	key := countsKey(p.Slug)
	exists, err := s.rdb.Exists(ctx, key).Result()
	if err != nil {
		return err
	}
	if exists == 1 {
		return nil
	}

	fields := make(map[string]interface{}, len(p.Options))
	for _, o := range p.Options {
		fields[o.ID] = p.Counts[o.ID]
	}
	pipe := s.rdb.TxPipeline()
	pipe.HSet(ctx, key, fields)
	pipe.Expire(ctx, key, keyTTL)
	_, err = pipe.Exec(ctx)
	return err
}

func (s *Service) Counts(ctx context.Context, p *models.Poll) (map[string]int64, int64, error) {
	if err := s.Rehydrate(ctx, p); err != nil {
		return nil, 0, err
	}
	raw, err := s.rdb.HGetAll(ctx, countsKey(p.Slug)).Result()
	if err != nil {
		return nil, 0, err
	}
	counts := normalise(p, raw)
	return counts, total(counts), nil
}

// Increment applies one vote. HINCRBY is atomic, so two people voting at the
// same instant can never read-modify-write over each other.
func (s *Service) Increment(ctx context.Context, p *models.Poll, optionIDs []string) (map[string]int64, int64, error) {
	if err := s.Rehydrate(ctx, p); err != nil {
		return nil, 0, err
	}
	key := countsKey(p.Slug)
	pipe := s.rdb.TxPipeline()
	for _, id := range optionIDs {
		pipe.HIncrBy(ctx, key, id, 1)
	}
	all := pipe.HGetAll(ctx, key)
	pipe.Expire(ctx, key, keyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return nil, 0, err
	}
	counts := normalise(p, all.Val())
	return counts, total(counts), nil
}

// normalise guarantees the map always contains exactly the poll's current
// options, so the frontend never has to defend against missing keys.
func normalise(p *models.Poll, raw map[string]string) map[string]int64 {
	out := make(map[string]int64, len(p.Options))
	for _, o := range p.Options {
		var n int64
		if v, ok := raw[o.ID]; ok {
			fmt.Sscanf(v, "%d", &n)
		}
		out[o.ID] = n
	}
	return out
}

func total(counts map[string]int64) int64 {
	var t int64
	for _, v := range counts {
		t += v
	}
	return t
}

// ---------- duplicate guard ----------

// ClaimVoter returns true if this voter had not voted yet. SADD does the
// check and the claim in one round trip, which is why a burst of duplicate
// requests cannot slip two votes through.
func (s *Service) ClaimVoter(ctx context.Context, slug, voterKey string) (bool, error) {
	pipe := s.rdb.TxPipeline()
	added := pipe.SAdd(ctx, votersKey(slug), voterKey)
	pipe.Expire(ctx, votersKey(slug), keyTTL)
	if _, err := pipe.Exec(ctx); err != nil {
		return false, err
	}
	return added.Val() == 1, nil
}

// ReleaseVoter undoes a claim when the durable write afterwards fails, so a
// database hiccup does not permanently lock someone out of voting.
func (s *Service) ReleaseVoter(ctx context.Context, slug, voterKey string) {
	s.rdb.SRem(ctx, votersKey(slug), voterKey)
}

func (s *Service) HasVoted(ctx context.Context, slug, voterKey string) bool {
	ok, err := s.rdb.SIsMember(ctx, votersKey(slug), voterKey).Result()
	return err == nil && ok
}

// SeedVoters restores the voter set from durable vote records after a flush.
func (s *Service) SeedVoters(ctx context.Context, slug string, keys []string) {
	if len(keys) == 0 {
		return
	}
	members := make([]interface{}, len(keys))
	for i, k := range keys {
		members[i] = k
	}
	pipe := s.rdb.TxPipeline()
	pipe.SAdd(ctx, votersKey(slug), members...)
	pipe.Expire(ctx, votersKey(slug), keyTTL)
	_, _ = pipe.Exec(ctx)
}

// ---------- fan-out ----------

func (s *Service) Publish(ctx context.Context, slug string, snap Snapshot) {
	b, err := json.Marshal(snap)
	if err != nil {
		return
	}
	s.rdb.Publish(ctx, channel(slug), b)
}

func (s *Service) Subscribe(ctx context.Context, slug string) *redis.PubSub {
	return s.rdb.Subscribe(ctx, channel(slug))
}

// ---------- rate limiting ----------

// Allow is a fixed-window counter. INCR on a fresh key returns 1, which is the
// signal to attach the window expiry - no separate existence check needed.
func (s *Service) Allow(ctx context.Context, bucket string, limit int64, window time.Duration) (bool, error) {
	key := "rate:" + bucket
	n, err := s.rdb.Incr(ctx, key).Result()
	if err != nil {
		return true, err // fail open: a Redis blip should not take voting down
	}
	if n == 1 {
		s.rdb.Expire(ctx, key, window)
	}
	return n <= limit, nil
}

// Viewers tracks how many live streams are attached to a poll, so the creator
// can see the room filling up before the first vote lands.
func (s *Service) Viewers(ctx context.Context, slug string, delta int64) int64 {
	key := "poll:" + slug + ":viewers"
	n, err := s.rdb.IncrBy(ctx, key, delta).Result()
	if err != nil {
		return 0
	}
	if n < 0 {
		s.rdb.Set(ctx, key, 0, keyTTL)
		return 0
	}
	s.rdb.Expire(ctx, key, 6*time.Hour)
	return n
}
