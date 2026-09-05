package repository

import (
	"context"
	"errors"
	"regexp"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"

	"jsts-salebot/internal/models"
)

// Fields is a set of document fields to $set.
type Fields = map[string]any

// UserRepository reads and writes the "users" collection.
type UserRepository struct {
	coll *mongo.Collection
}

// NewUserRepository binds to db.users.
func NewUserRepository(db *mongo.Database) *UserRepository {
	return &UserRepository{coll: db.Collection("users")}
}

func (r *UserRepository) findOne(ctx context.Context, filter any) (*models.User, error) {
	var u models.User
	err := r.coll.FindOne(ctx, filter).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// FindByUserID returns nil, nil for an unknown user.
func (r *UserRepository) FindByUserID(ctx context.Context, userID string) (*models.User, error) {
	return r.findOne(ctx, bson.M{"userId": userID})
}

// FindByUsername matches a Telegram username case-insensitively.
func (r *UserRepository) FindByUsername(ctx context.Context, userName string) (*models.User, error) {
	pattern := "^" + regexp.QuoteMeta(userName) + "$"
	return r.findOne(ctx, bson.M{"userName": bson.Regex{Pattern: pattern, Options: "i"}})
}

// FindManyByIDs fetches the users whose userId is in ids.
func (r *UserRepository) FindManyByIDs(ctx context.Context, ids []string) ([]models.User, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	cur, err := r.coll.Find(ctx, bson.M{"userId": bson.M{"$in": ids}})
	if err != nil {
		return nil, err
	}
	var users []models.User
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// UpsertUserWithInsert creates or refreshes a user. set holds the fields to
// write on every call; the Mongoose defaults (null names, authLevel 0,
// timestamps) are applied only on insert and never overlap set, which would
// trigger a ConflictingUpdateOperators error. Documents still carrying the
// legacy isAdmin flag are migrated to authLevel here.
func (r *UserRepository) UpsertUserWithInsert(ctx context.Context, userID string, set Fields) (*models.User, error) {
	s := bson.M{}
	for k, v := range set {
		s[k] = v
	}
	s["userId"] = userID

	// Legacy migration: isAdmin (boolean) becomes authLevel.
	var existing bson.M
	err := r.coll.FindOne(ctx, bson.M{"userId": userID}).Decode(&existing)
	if err != nil && !errors.Is(err, mongo.ErrNoDocuments) {
		return nil, err
	}
	update := bson.M{}
	if isAdmin, ok := existing["isAdmin"].(bool); ok {
		if isAdmin {
			s["authLevel"] = models.AuthAdmin
		} else {
			s["authLevel"] = models.AuthUser
		}
		update["$unset"] = bson.M{"isAdmin": 1}
	}

	now := time.Now()
	s["updatedAt"] = now
	setOnInsert := bson.M{"createdAt": now}
	for field, def := range map[string]any{
		"firstName": nil, "lastName": nil, "userName": nil,
		"languageCode": nil, "preferredLocale": nil,
		"authLevel": models.AuthUser,
	} {
		if _, present := s[field]; !present {
			setOnInsert[field] = def
		}
	}
	update["$set"] = s
	update["$setOnInsert"] = setOnInsert

	opts := options.FindOneAndUpdate().SetUpsert(true).SetReturnDocument(options.After)
	var u models.User
	if err := r.coll.FindOneAndUpdate(ctx, bson.M{"userId": userID}, update, opts).Decode(&u); err != nil {
		return nil, err
	}
	return &u, nil
}

// UpdateUser applies a $set; changing authLevel also drops the legacy isAdmin.
func (r *UserRepository) UpdateUser(ctx context.Context, userID string, set Fields) (*models.User, error) {
	update := bson.M{"$set": bson.M(set)}
	if _, ok := set["authLevel"]; ok {
		update["$unset"] = bson.M{"isAdmin": 1}
	}
	opts := options.FindOneAndUpdate().SetReturnDocument(options.After)
	var u models.User
	err := r.coll.FindOneAndUpdate(ctx, bson.M{"userId": userID}, update, opts).Decode(&u)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &u, nil
}

// GetAll returns every user.
func (r *UserRepository) GetAll(ctx context.Context) ([]models.User, error) {
	cur, err := r.coll.Find(ctx, bson.M{})
	if err != nil {
		return nil, err
	}
	var users []models.User
	if err := cur.All(ctx, &users); err != nil {
		return nil, err
	}
	return users, nil
}

// CountByAuthLevel counts users holding exactly level.
func (r *UserRepository) CountByAuthLevel(ctx context.Context, level models.AuthLevel) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"authLevel": level})
}

// CountLegacyIsAdmin counts documents still carrying the pre-RBAC isAdmin flag.
func (r *UserRepository) CountLegacyIsAdmin(ctx context.Context) (int64, error) {
	return r.coll.CountDocuments(ctx, bson.M{"isAdmin": bson.M{"$exists": true}})
}

// DeleteByUserID removes a user, reporting whether one existed.
func (r *UserRepository) DeleteByUserID(ctx context.Context, userID string) (bool, error) {
	res, err := r.coll.DeleteOne(ctx, bson.M{"userId": userID})
	if err != nil {
		return false, err
	}
	return res.DeletedCount > 0, nil
}

// HasAuthLevel reports whether the user holds at least level.
func (r *UserRepository) HasAuthLevel(ctx context.Context, userID string, level models.AuthLevel) (bool, error) {
	u, err := r.FindByUserID(ctx, userID)
	if err != nil {
		return false, err
	}
	return models.Level(u) >= level, nil
}
