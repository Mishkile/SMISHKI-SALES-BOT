// Package models holds the persisted document shapes. The bson tags match the
// collections written by the original TypeScript/Mongoose implementation, so a
// database created by either version can be used by the other.
package models

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// AuthLevel is the role stored on a user: 0 = user, 1 = moderator, 2 = admin.
type AuthLevel int

const (
	AuthUser  AuthLevel = 0
	AuthMod   AuthLevel = 1
	AuthAdmin AuthLevel = 2
)

// MediaType distinguishes photos from videos in a post's media list.
type MediaType string

const (
	MediaPhoto MediaType = "photo"
	MediaVideo MediaType = "video"
)

// MediaItem is one uploaded photo or video, referenced by Telegram file_id.
type MediaItem struct {
	FileID string    `bson:"fileId" json:"fileId"`
	Type   MediaType `bson:"type" json:"type"`
}

// PostStatus is the moderation lifecycle state of a post.
type PostStatus string

const (
	StatusPending  PostStatus = "pending"
	StatusApproved PostStatus = "approved"
	StatusRejected PostStatus = "rejected"
	StatusSold     PostStatus = "sold"
)

// Post is a sale listing (collection "posts").
type Post struct {
	ID                  bson.ObjectID `bson:"_id,omitempty"`
	UserID              string        `bson:"userId"`
	Status              PostStatus    `bson:"status"`
	Price               string        `bson:"price"`
	Title               string        `bson:"title"`
	Description         string        `bson:"description"`
	Location            string        `bson:"location"`
	Media               []MediaItem   `bson:"media"`
	CreatedAt           time.Time     `bson:"createdAt"`
	IsExpired           bool          `bson:"isExpired"`
	LastBumpAt          *time.Time    `bson:"lastBumpAt"`
	DailyBumpCount      int           `bson:"dailyBumpCount"`
	ApprovedMessageID   *int          `bson:"approvedMessageId"`
	ModerationMessageID *int          `bson:"moderationMessageId"`
	RejectionReason     *string       `bson:"rejectionReason"`
}

// User is a Telegram user known to the bot (collection "users").
type User struct {
	UserID          string    `bson:"userId"`
	FirstName       *string   `bson:"firstName"`
	LastName        *string   `bson:"lastName"`
	UserName        *string   `bson:"userName"`
	AuthLevel       AuthLevel `bson:"authLevel"`
	LanguageCode    *string   `bson:"languageCode"`
	PreferredLocale *string   `bson:"preferredLocale"`
	CreatedAt       time.Time `bson:"createdAt,omitempty"`
	UpdatedAt       time.Time `bson:"updatedAt,omitempty"`
}

// Str dereferences an optional string, returning "" when unset.
func Str(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

// StrOr dereferences an optional string, returning def when unset or empty.
func StrOr(p *string, def string) string {
	if p == nil || *p == "" {
		return def
	}
	return *p
}

// Ptr returns a pointer to v; handy for the nullable document fields.
func Ptr[T any](v T) *T { return &v }

// Level returns the user's auth level, treating an unknown user as a plain user.
func Level(u *User) AuthLevel {
	if u == nil {
		return AuthUser
	}
	return u.AuthLevel
}
