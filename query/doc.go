// Package query provides a type-safe, generic query builder for constructing
// SQL queries with compile-time safety.
//
// The central type is [Builder], a generic query builder that supports
// filtering, ordering, pagination, column selection, joins, and eager loading.
// Each method returns an immutable clone, allowing safe reuse of partial queries.
//
// Type-safe fields ([StringField], [IntField], [BoolField], etc.) ensure that
// filter conditions are validated at compile time, preventing runtime SQL errors.
//
// Basic usage:
//
//	users, err := query.NewBuilder[User](executor, dialect, "users").
//	    Where(UserFields.Name.Eq("Alice")).
//	    OrderByAsc("created_at").
//	    Limit(10).
//	    Find(ctx)
package query
