// Package core provides the foundational types and functions for the Genus ORM.
//
// The central type is [DB], which wraps a database connection and provides
// type-safe CRUD operations using Go generics. It supports logging, hooks,
// read replicas, and audit trails.
//
// Models embed [Model] to get standard fields (ID, CreatedAt, UpdatedAt) and
// can implement lifecycle hooks like [BeforeCreater] and [AfterFinder].
//
// Basic usage:
//
//	db, err := core.New(sqlDB, dialect)
//	err = db.Create(ctx, &user)
//	users, err := db.Find(ctx, conditions...)
package core
