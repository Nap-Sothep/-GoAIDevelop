package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
	"go.mongodb.org/mongo-driver/v2/mongo"
	"go.mongodb.org/mongo-driver/v2/mongo/options"
	"go.uber.org/zap"

	"go-gateway/internal/model"
)

// UserRepository 用户数据访问层
type UserRepository struct {
	coll *mongo.Collection
}

// NewUserRepository 创建用户Repository（自动创建必要索引）
func NewUserRepository(db *mongo.Database) *UserRepository {
	coll := db.Collection(model.User{}.CollectionName())

	// 创建必要索引
	indexes := []mongo.IndexModel{
		{
			Keys:    bson.M{"name": 1},
			Options: options.Index().SetUnique(true),
		},
		{
			Keys:    bson.M{"email": 1},
			Options: options.Index().SetUnique(true).SetSparse(true),
		},
		{
			Keys: bson.M{"created_at": -1},
		},
	}

	_, err := coll.Indexes().CreateMany(context.Background(), indexes)
	if err != nil {
		zap.L().Warn("创建索引失败（可能已存在）", zap.Error(err))
	} else {
		zap.L().Info("MongoDB索引创建成功")
	}

	return &UserRepository{
		coll: coll,
	}
}

// Create 创建用户
func (r *UserRepository) Create(ctx context.Context, user *model.User) error {
	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	result, err := r.coll.InsertOne(ctx, user)
	if err != nil {
		return fmt.Errorf("创建用户失败 [name=%s]: %w", user.Name, err)
	}

	// 将插入的ID转换回ObjectID
	if oid, ok := result.InsertedID.(bson.ObjectID); ok {
		user.ID = oid
	}

	zap.L().Debug("用户创建成功", zap.String("id", user.ID.Hex()), zap.String("name", user.Name))
	return nil
}

// GetByID 根据ID查询用户
func (r *UserRepository) GetByID(ctx context.Context, id string) (*model.User, error) {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return nil, fmt.Errorf("无效的用户ID [%s]: %w", id, err)
	}

	var user model.User
	filter := bson.M{"_id": objectID}
	err = r.coll.FindOne(ctx, filter).Decode(&user)

	// 规则M5: 必须使用 ErrNoDocuments 判空
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil // 用户不存在，返回 nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败 [id=%s]: %w", id, err)
	}

	return &user, nil
}

// GetByName 根据名称查询用户
func (r *UserRepository) GetByName(ctx context.Context, name string) (*model.User, error) {
	var user model.User
	filter := bson.M{"name": name}
	err := r.coll.FindOne(ctx, filter).Decode(&user)

	// 规则M5: 必须使用 ErrNoDocuments 判空
	if errors.Is(err, mongo.ErrNoDocuments) {
		return nil, nil // 用户不存在
	}
	if err != nil {
		return nil, fmt.Errorf("查询用户失败 [name=%s]: %w", name, err)
	}

	return &user, nil
}

// Update 更新用户
func (r *UserRepository) Update(ctx context.Context, user *model.User) error {
	if user.ID.IsZero() {
		return fmt.Errorf("用户ID不能为空")
	}

	user.UpdatedAt = time.Now()
	filter := bson.M{"_id": user.ID}
	update := bson.M{
		"$set": bson.M{
			"name":       user.Name,
			"email":      user.Email,
			"age":        user.Age,
			"updated_at": user.UpdatedAt,
		},
	}

	result, err := r.coll.UpdateOne(ctx, filter, update)
	if err != nil {
		return fmt.Errorf("更新用户失败 [id=%s]: %w", user.ID.Hex(), err)
	}

	if result.MatchedCount == 0 {
		return fmt.Errorf("用户不存在 [id=%s]", user.ID.Hex())
	}

	zap.L().Debug("用户更新成功", zap.String("id", user.ID.Hex()))
	return nil
}

// Delete 删除用户
func (r *UserRepository) Delete(ctx context.Context, id string) error {
	objectID, err := bson.ObjectIDFromHex(id)
	if err != nil {
		return fmt.Errorf("无效的用户ID [%s]: %w", id, err)
	}

	filter := bson.M{"_id": objectID}
	result, err := r.coll.DeleteOne(ctx, filter)
	if err != nil {
		return fmt.Errorf("删除用户失败 [id=%s]: %w", id, err)
	}

	if result.DeletedCount == 0 {
		return fmt.Errorf("用户不存在 [id=%s]", id)
	}

	zap.L().Debug("用户删除成功", zap.String("id", id))
	return nil
}

// List 分页查询用户列表
func (r *UserRepository) List(ctx context.Context, page, pageSize int64) ([]*model.User, int64, error) {
	// 参数校验
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 10
	}

	skip := (page - 1) * pageSize

	// 查询总数
	total, err := r.coll.CountDocuments(ctx, bson.M{})
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户总数失败: %w", err)
	}

	// 分页查询
	opts := options.Find().
		SetSkip(skip).
		SetLimit(pageSize).
		SetSort(bson.M{"created_at": -1}) // 按创建时间倒序

	cursor, err := r.coll.Find(ctx, bson.M{}, opts)
	if err != nil {
		return nil, 0, fmt.Errorf("查询用户列表失败: %w", err)
	}
	defer cursor.Close(ctx)

	var users []*model.User
	if err := cursor.All(ctx, &users); err != nil {
		return nil, 0, fmt.Errorf("解析用户列表失败: %w", err)
	}

	return users, total, nil
}

// BatchCreate 批量创建用户（规则M6: 使用 BulkWrite）
func (r *UserRepository) BatchCreate(ctx context.Context, users []*model.User) error {
	if len(users) == 0 {
		return nil
	}

	models := make([]mongo.WriteModel, 0, len(users))
	now := time.Now()

	for _, user := range users {
		user.CreatedAt = now
		user.UpdatedAt = now
		models = append(models, mongo.NewInsertOneModel().SetDocument(user))
	}

	_, err := r.coll.BulkWrite(ctx, models)
	if err != nil {
		return fmt.Errorf("批量创建用户失败: %w", err)
	}

	zap.L().Debug("批量创建用户成功", zap.Int("count", len(users)))
	return nil
}
