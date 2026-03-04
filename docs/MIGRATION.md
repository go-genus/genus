# Migrating from GORM to Genus

This guide helps you migrate an existing GORM codebase to Genus. The migration can be done incrementally - both ORMs can coexist during the transition.

## Why Migrate?

| Aspect | GORM | Genus |
|--------|------|-------|
| Query errors | Runtime | Compile-time |
| Performance | Baseline | 1.6x faster |
| Memory allocations | 7,440/query | 3,900/query |
| IDE autocomplete | Limited | Full support |
| Dependencies | Multiple | Zero |

## Quick Reference

| GORM | Genus |
|------|-------|
| `db.Where("name = ?", "Alice").Find(&users)` | `genus.Table[User](db).Where(UserFields.Name.Eq("Alice")).Find(ctx)` |
| `db.First(&user)` | `genus.Table[User](db).First(ctx)` |
| `db.Create(&user)` | `db.DB().Create(ctx, &user)` |
| `db.Save(&user)` | `db.DB().Update(ctx, &user)` |
| `db.Delete(&user)` | `db.DB().Delete(ctx, &user)` |

## Step-by-Step Migration

### 1. Models

#### GORM Model

```go
type User struct {
    gorm.Model
    Name     string `gorm:"column:name"`
    Email    string `gorm:"column:email;uniqueIndex"`
    Age      int    `gorm:"column:age"`
    IsActive bool   `gorm:"column:is_active;default:true"`
}
```

#### Genus Model

```go
type User struct {
    core.Model
    Name     string `db:"name"`
    Email    string `db:"email"`
    Age      int    `db:"age"`
    IsActive bool   `db:"is_active"`
}

// Add type-safe fields
var UserFields = struct {
    ID       query.Int64Field
    Name     query.StringField
    Email    query.StringField
    Age      query.IntField
    IsActive query.BoolField
}{
    ID:       query.NewInt64Field("id"),
    Name:     query.NewStringField("name"),
    Email:    query.NewStringField("email"),
    Age:      query.NewIntField("age"),
    IsActive: query.NewBoolField("is_active"),
}
```

**Key differences:**
- Replace `gorm.Model` with `core.Model`
- Replace `gorm:"column:x"` tags with `db:"x"`
- Add typed fields for compile-time safety

### 2. Database Connection

#### GORM

```go
import "gorm.io/gorm"
import "gorm.io/driver/postgres"

db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
```

#### Genus

```go
import "github.com/go-genus/genus"
import _ "github.com/lib/pq"

db, err := genus.Open("postgres", dsn)
```

### 3. Basic Queries

#### Find All

```go
// GORM
var users []User
db.Find(&users)

// Genus
users, err := genus.Table[User](db).Find(ctx)
```

#### Find with Conditions

```go
// GORM
var users []User
db.Where("is_active = ?", true).Where("age > ?", 18).Find(&users)

// Genus
users, err := genus.Table[User](db).
    Where(UserFields.IsActive.Eq(true)).
    Where(UserFields.Age.Gt(18)).
    Find(ctx)
```

#### Find First

```go
// GORM
var user User
db.First(&user)

// Genus
user, err := genus.Table[User](db).First(ctx)
```

#### Find by ID

```go
// GORM
var user User
db.First(&user, 1)

// Genus
user, err := genus.Table[User](db).
    Where(UserFields.ID.Eq(1)).
    First(ctx)
```

### 4. CRUD Operations

#### Create

```go
// GORM
user := User{Name: "Alice", Email: "alice@example.com"}
db.Create(&user)

// Genus
user := &User{Name: "Alice", Email: "alice@example.com"}
err := db.DB().Create(ctx, user)
```

#### Update

```go
// GORM - Update single field
db.Model(&user).Update("name", "Alice Smith")

// GORM - Update multiple fields
db.Model(&user).Updates(User{Name: "Alice", Age: 30})

// GORM - Save all fields
db.Save(&user)

// Genus - Update all fields
user.Name = "Alice Smith"
err := db.DB().Update(ctx, user)
```

#### Delete

```go
// GORM
db.Delete(&user)

// Genus
err := db.DB().Delete(ctx, user)
```

### 5. Query Building

#### Order By

```go
// GORM
db.Order("created_at desc").Find(&users)

// Genus
users, err := genus.Table[User](db).
    OrderByDesc("created_at").
    Find(ctx)
```

#### Limit and Offset

```go
// GORM
db.Limit(10).Offset(20).Find(&users)

// Genus
users, err := genus.Table[User](db).
    Limit(10).
    Offset(20).
    Find(ctx)
```

#### Select Specific Columns

```go
// GORM
db.Select("name", "email").Find(&users)

// Genus
users, err := genus.Table[User](db).
    Select("name", "email").
    Find(ctx)
```

#### Count

```go
// GORM
var count int64
db.Model(&User{}).Where("is_active = ?", true).Count(&count)

// Genus
count, err := genus.Table[User](db).
    Where(UserFields.IsActive.Eq(true)).
    Count(ctx)
```

### 6. Advanced Queries

#### OR Conditions

```go
// GORM
db.Where("name = ?", "Alice").Or("name = ?", "Bob").Find(&users)

// Genus
users, err := genus.Table[User](db).
    Where(query.Or(
        UserFields.Name.Eq("Alice"),
        UserFields.Name.Eq("Bob"),
    )).
    Find(ctx)
```

#### IN Clause

```go
// GORM
db.Where("name IN ?", []string{"Alice", "Bob"}).Find(&users)

// Genus
users, err := genus.Table[User](db).
    Where(UserFields.Name.In([]string{"Alice", "Bob"})).
    Find(ctx)
```

#### LIKE

```go
// GORM
db.Where("name LIKE ?", "%alice%").Find(&users)

// Genus
users, err := genus.Table[User](db).
    Where(UserFields.Name.Like("%alice%")).
    Find(ctx)
```

#### BETWEEN

```go
// GORM
db.Where("age BETWEEN ? AND ?", 18, 65).Find(&users)

// Genus
users, err := genus.Table[User](db).
    Where(UserFields.Age.Between(18, 65)).
    Find(ctx)
```

### 7. Relationships

#### Define Relationships

```go
// GORM
type User struct {
    gorm.Model
    Name  string
    Posts []Post `gorm:"foreignKey:UserID"`
}

type Post struct {
    gorm.Model
    Title  string
    UserID uint
    User   User `gorm:"foreignKey:UserID"`
}

// Genus
type User struct {
    core.Model
    Name  string `db:"name"`
    Posts []Post `db:"-" relation:"has_many,foreign_key=user_id"`
}

type Post struct {
    core.Model
    Title  string `db:"title"`
    UserID int64  `db:"user_id"`
    User   *User  `db:"-" relation:"belongs_to,foreign_key=user_id"`
}
```

#### Eager Loading

```go
// GORM
db.Preload("Posts").Find(&users)

// Genus
genus.RegisterModels(&User{}, &Post{})  // Once at startup

users, err := genus.Table[User](db).
    Preload("Posts").
    Find(ctx)
```

### 8. Transactions

```go
// GORM
db.Transaction(func(tx *gorm.DB) error {
    if err := tx.Create(&user).Error; err != nil {
        return err
    }
    if err := tx.Create(&post).Error; err != nil {
        return err
    }
    return nil
})

// Genus
db.DB().WithTx(ctx, func(tx *core.DB) error {
    if err := tx.Create(ctx, &user); err != nil {
        return err
    }
    if err := tx.Create(ctx, &post); err != nil {
        return err
    }
    return nil
})
```

### 9. Hooks

```go
// GORM
func (u *User) BeforeCreate(tx *gorm.DB) error {
    u.CreatedAt = time.Now()
    return nil
}

// Genus
func (u *User) BeforeCreate() error {
    // CreatedAt is set automatically by core.Model
    return nil
}
```

**Available hooks in Genus:**
- `BeforeCreate()`, `AfterCreate()`
- `BeforeUpdate()`, `AfterUpdate()`
- `BeforeDelete()`, `AfterDelete()`
- `BeforeSave()`, `AfterSave()`
- `AfterFind()`

### 10. Soft Deletes

```go
// GORM
type User struct {
    gorm.Model  // Includes DeletedAt
    Name string
}

db.Delete(&user)           // Soft delete
db.Unscoped().Delete(&user) // Hard delete

// Genus
type User struct {
    core.SoftDeleteModel  // Includes DeletedAt
    Name string `db:"name"`
}

db.DB().Delete(ctx, &user)       // Soft delete
db.DB().ForceDelete(ctx, &user)  // Hard delete

// Query soft-deleted records
genus.Table[User](db).WithTrashed().Find(ctx)     // Include deleted
genus.Table[User](db).OnlyTrashed().Find(ctx)     // Only deleted
```

### 11. Raw SQL

```go
// GORM
var users []User
db.Raw("SELECT * FROM users WHERE age > ?", 18).Scan(&users)

// Genus - Use standard database/sql
rows, err := db.DB().Executor().QueryContext(ctx,
    "SELECT * FROM users WHERE age > $1", 18)
```

### 12. Migrations

```go
// GORM
db.AutoMigrate(&User{}, &Post{})

// Genus
import "github.com/go-genus/genus/migrate"

migrate.AutoMigrate(ctx, db.DB(), db.DB().Dialect(), User{}, Post{})
```

## Incremental Migration Strategy

You can run GORM and Genus side by side:

```go
import (
    "gorm.io/gorm"
    "github.com/go-genus/genus"
)

func main() {
    // Keep GORM for existing code
    gormDB, _ := gorm.Open(postgres.Open(dsn), &gorm.Config{})

    // Add Genus for new code
    sqlDB, _ := gormDB.DB()  // Get underlying *sql.DB
    genusDB := genus.NewWithLogger(sqlDB, dialects.PostgreSQL{}, &core.NoOpLogger{})

    // Migrate gradually:
    // 1. New features use Genus
    // 2. Refactor existing code over time
    // 3. Remove GORM when migration is complete
}
```

## Common Gotchas

### 1. Context is Required

GORM doesn't require context, Genus does:

```go
// GORM
db.Find(&users)

// Genus - context is required
users, err := genus.Table[User](db).Find(ctx)
```

### 2. Error Handling

```go
// GORM - errors are on db.Error
result := db.Find(&users)
if result.Error != nil {
    // handle error
}

// Genus - errors are returned directly
users, err := genus.Table[User](db).Find(ctx)
if err != nil {
    // handle error
}
```

### 3. Pointer vs Value

```go
// GORM accepts both
db.Create(&user)
db.Create(user)

// Genus requires pointer for Create/Update/Delete
db.DB().Create(ctx, &user)  // Correct
db.DB().Create(ctx, user)   // Won't work
```

### 4. Model Registration for Relationships

```go
// GORM - automatic
db.Preload("Posts").Find(&users)

// Genus - requires registration
genus.RegisterModels(&User{}, &Post{})  // Do this once at startup
genus.Table[User](db).Preload("Posts").Find(ctx)
```

## Performance Tips After Migration

1. **Use GenusUltra for hot paths:**
   ```go
   genus.UltraFastTable[User](db).Find(ctx)
   ```

2. **Use batch operations:**
   ```go
   db.DB().BatchInsert(ctx, users)
   ```

3. **Configure connection pooling:**
   ```go
   genus.OpenWithConfig("postgres", dsn, core.HighPerformancePoolConfig())
   ```

## Getting Help

- [GitHub Issues](https://github.com/go-genus/genus/issues)
- [GitHub Discussions](https://github.com/go-genus/genus/discussions)
