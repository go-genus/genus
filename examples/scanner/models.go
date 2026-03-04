package scanner

import (
	"github.com/go-genus/genus/core"
)

// User model for demonstrating generated scanners.
type User struct {
	core.Model
	Name     string `db:"name"`
	Email    string `db:"email"`
	Age      int    `db:"age"`
	IsActive bool   `db:"is_active"`
	Score    int    `db:"score"`
}

// TableName returns the table name for User.
func (User) TableName() string { return "users" }

// Post model for demonstrating generated scanners.
type Post struct {
	core.Model
	Title   string `db:"title"`
	Content string `db:"content"`
	UserID  int64  `db:"user_id"`
	Views   int    `db:"views"`
}

// Comment model for demonstrating generated scanners.
type Comment struct {
	core.Model
	PostID  int64  `db:"post_id"`
	UserID  int64  `db:"user_id"`
	Content string `db:"content"`
}
