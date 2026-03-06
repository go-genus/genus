// Package cache provides a pluggable query caching layer for the Genus ORM.
//
// The [Cache] interface defines operations for Get, Set, Delete, and
// DeleteByPrefix, along with cache statistics. The built-in [InMemoryCache]
// implementation supports TTL expiration, max entry limits, and hit/miss tracking.
//
// Use [CachedBuilder] to wrap a query builder with automatic cache lookups.
package cache
