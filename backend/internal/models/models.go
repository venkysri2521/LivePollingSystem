package models

import (
	"time"

	"go.mongodb.org/mongo-driver/bson/primitive"
)

type User struct {
	ID           primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	Name         string             `bson:"name"          json:"name"`
	Email        string             `bson:"email"         json:"email"`
	PasswordHash string             `bson:"passwordHash"  json:"-"` // never leaves the server
	CreatedAt    time.Time          `bson:"createdAt"     json:"createdAt"`
}

type Option struct {
	ID   string `bson:"id"   json:"id"`
	Text string `bson:"text" json:"text"`
}

type Poll struct {
	ID            primitive.ObjectID `bson:"_id,omitempty"  json:"id"`
	Slug          string             `bson:"slug"           json:"slug"`
	Question      string             `bson:"question"       json:"question"`
	Options       []Option           `bson:"options"        json:"options"`
	OwnerID       primitive.ObjectID `bson:"ownerId"        json:"ownerId"`
	OwnerName     string             `bson:"ownerName"      json:"ownerName"`
	Status        string             `bson:"status"         json:"status"` // open | closed
	AllowMultiple bool               `bson:"allowMultiple"  json:"allowMultiple"`
	HideUntilVote bool               `bson:"hideUntilVote"  json:"hideUntilVote"`
	ClosesAt      *time.Time         `bson:"closesAt,omitempty" json:"closesAt,omitempty"`
	Counts        map[string]int64   `bson:"counts"         json:"counts"`
	TotalVotes    int64              `bson:"totalVotes"     json:"totalVotes"`
	CreatedAt     time.Time          `bson:"createdAt"      json:"createdAt"`
}

const (
	StatusOpen   = "open"
	StatusClosed = "closed"
)

// IsAcceptingVotes folds the two ways a poll can stop taking votes into one
// check, so handlers never have to remember both.
func (p *Poll) IsAcceptingVotes(now time.Time) bool {
	if p.Status != StatusOpen {
		return false
	}
	if p.ClosesAt != nil && now.After(*p.ClosesAt) {
		return false
	}
	return true
}

func (p *Poll) HasOption(id string) bool {
	for _, o := range p.Options {
		if o.ID == id {
			return true
		}
	}
	return false
}

type Vote struct {
	ID        primitive.ObjectID `bson:"_id,omitempty" json:"id"`
	PollID    primitive.ObjectID `bson:"pollId"        json:"pollId"`
	OptionIDs []string           `bson:"optionIds"     json:"optionIds"`
	VoterKey  string             `bson:"voterKey"      json:"-"` // hashed, never returned
	CreatedAt time.Time          `bson:"createdAt"     json:"createdAt"`
}
