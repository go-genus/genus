// Package sharding provides horizontal data distribution across multiple
// database shards.
//
// The [ShardKey] interface and [ShardStrategy] interface allow pluggable
// sharding logic. Built-in strategies include [ModuloStrategy] for simple
// hash-based distribution and consistent hashing for minimal key redistribution.
//
// Shard keys can be integers ([Int64ShardKey]) or strings ([StringShardKey]).
package sharding
