package model

import (
	"time"

	"go.mongodb.org/mongo-driver/v2/bson"
)

// User 用户模型
type User struct {
	ID        bson.ObjectID `bson:"_id,omitempty" json:"id"`
	Name      string        `bson:"name" json:"name"`
	Email     string        `bson:"email" json:"email"`
	Age       int           `bson:"age" json:"age"`
	CreatedAt time.Time     `bson:"created_at" json:"created_at"`
	UpdatedAt time.Time     `bson:"updated_at" json:"updated_at"`
}

// TableName 返回集合名称
func (User) CollectionName() string {
	return "users"
}
