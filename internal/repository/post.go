// Package repository is the only layer that talks to MongoDB.
package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"jsts-salebot/internal/models"
)

// PostRepository reads and writes the "posts" collection.
type PostRepository struct {
	coll *mongo.Collection
}

// NewPostRepository binds to db.posts.
func NewPostRepository(db *mongo.Database) *PostRepository {
	return &PostRepository{coll: db.Collection("posts")}
}

// NewPost is the input for Create; zero values take the Mongoose defaults.
type NewPost struct {
	UserID      string
	Title       string
	Description string
	Price       string
	Location    string
	Media       []models.MediaItem
	Status      models.PostStatus
	CreatedAt   time.Time
}

func parseID(id string) (bson.ObjectID, error) {
	oid, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return bson.ObjectID{}, fmt.Errorf("invalid post id %q: %w", id, err)
	}
	return oid, nil
}

// Create inserts a post and returns it with its generated id.
func (r *PostRepository) Create(ctx context.Context, in NewPost) (*models.Post, error) {
	post := models.Post{
		ID:          bson.NewObjectID(),
		UserID:      in.UserID,
		Status:      in.Status,
		Price:       in.Price,
		Title:       in.Title,
		Description: in.Description,
		Location:    in.Location,
		Media:       in.Media,
		CreatedAt:   in.CreatedAt,
	}
	if post.Status == "" {
		post.Status = models.StatusPending
	}
	if post.Media == nil {
		post.Media = []models.MediaItem{}
	}
	if post.CreatedAt.IsZero() {
		post.CreatedAt = time.Now()
	}
	if _, err := r.coll.InsertOne(ctx, post); err != nil {
		return nil, err
	}
	return &post, nil
}

func (r *PostRepository) find(ctx context.Context, filter any, sort bson.D) ([]models.Post, error) {
	opts := options.Find()
	if sort != nil {
		opts.SetSort(sort)
	}
	cur, err := r.coll.Find(ctx, filter, opts)
	if err != nil {
		return nil, err
	}
	var posts []models.Post
	if err := cur.All(ctx, &posts); err != nil {
		return nil, err
	}
	return posts, nil
}

func (r *PostRepository) findOne(ctx context.Context, filter any) (*models.Post, error) {
	var post models.Post
	err := r.coll.FindOne(ctx, filter).Decode(&post)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &post, nil
}

// FindByUserID lists a user's posts, newest first.
func (r *PostRepository) FindByUserID(ctx context.Context, userID string) ([]models.Post, error) {
	return r.find(ctx, bson.M{"userId": userID}, bson.D{{Key: "createdAt", Value: -1}})
}

// FindByID returns nil, nil when the post does not exist.
func (r *PostRepository) FindByID(ctx context.Context, id string) (*models.Post, error) {
	oid, err := parseID(id)
	if err != nil {
		return nil, err
	}
	return r.findOne(ctx, bson.M{"_id": oid})
}

// FindByApprovedMessageID looks a post up by its message in the approved group.
func (r *PostRepository) FindByApprovedMessageID(ctx context.Context, messageID int) (*models.Post, error) {
	return r.findOne(ctx, bson.M{"approvedMessageId": messageID})
}

func (r *PostRepository) updateByID(ctx context.Context, id string, set bson.M) error {
	oid, err := parseID(id)
	if err != nil {
		return err
	}
	_, err = r.coll.UpdateOne(ctx, bson.M{"_id": oid}, bson.M{"$set": set})
	return err
}

// UpdateStatus moves a post through its lifecycle.
func (r *PostRepository) UpdateStatus(ctx context.Context, id string, status models.PostStatus) error {
	return r.updateByID(ctx, id, bson.M{"status": status})
}

// SetApprovedMessageID records (or clears, with nil) the approved-group message.
func (r *PostRepository) SetApprovedMessageID(ctx context.Context, id string, messageID *int) error {
	return r.updateByID(ctx, id, bson.M{"approvedMessageId": messageID})
}

// SetModerationMessageID records (or clears, with nil) the moderation message.
func (r *PostRepository) SetModerationMessageID(ctx context.Context, id string, messageID *int) error {
	return r.updateByID(ctx, id, bson.M{"moderationMessageId": messageID})
}

// UpdateBump stamps the bump time and stores the new daily count.
func (r *PostRepository) UpdateBump(ctx context.Context, id string, dailyBumpCount int) error {
	return r.updateByID(ctx, id, bson.M{"lastBumpAt": time.Now(), "dailyBumpCount": dailyBumpCount})
}

// GetAll returns every post.
func (r *PostRepository) GetAll(ctx context.Context) ([]models.Post, error) {
	return r.find(ctx, bson.M{}, nil)
}

// GetPending returns pending posts in insertion order.
func (r *PostRepository) GetPending(ctx context.Context) ([]models.Post, error) {
	return r.find(ctx, bson.M{"status": models.StatusPending}, nil)
}

// GetPendingPosts returns pending posts, oldest first.
func (r *PostRepository) GetPendingPosts(ctx context.Context) ([]models.Post, error) {
	return r.find(ctx, bson.M{"status": models.StatusPending}, bson.D{{Key: "createdAt", Value: 1}})
}

// ExpireAllPendingPosts rejects every pending post (used by /clearpending).
func (r *PostRepository) ExpireAllPendingPosts(ctx context.Context) (int64, error) {
	res, err := r.coll.UpdateMany(ctx,
		bson.M{"status": models.StatusPending},
		bson.M{"$set": bson.M{"status": models.StatusRejected, "rejectionReason": "Expired via /clearpending"}},
	)
	if err != nil {
		return 0, err
	}
	return res.ModifiedCount, nil
}

// GetSold returns sold posts that still reference an approved-group message.
func (r *PostRepository) GetSold(ctx context.Context) ([]models.Post, error) {
	return r.find(ctx, bson.M{"status": models.StatusSold, "approvedMessageId": bson.M{"$ne": nil}}, nil)
}

// DistinctUserIDsByStatus lists the authors of posts in the given status.
func (r *PostRepository) DistinctUserIDsByStatus(ctx context.Context, status models.PostStatus) ([]string, error) {
	var ids []string
	if err := r.coll.Distinct(ctx, "userId", bson.M{"status": status}).Decode(&ids); err != nil {
		return nil, err
	}
	return ids, nil
}

// DeleteByID removes a post.
func (r *PostRepository) DeleteByID(ctx context.Context, id string) error {
	oid, err := parseID(id)
	if err != nil {
		return err
	}
	_, err = r.coll.DeleteOne(ctx, bson.M{"_id": oid})
	return err
}
