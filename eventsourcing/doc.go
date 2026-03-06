// Package eventsourcing provides event sourcing primitives for the Genus ORM.
//
// [Event] represents a domain event tied to an aggregate, with versioning
// and arbitrary data. [EventStore] persists and retrieves events from the
// database, enabling state reconstruction from the full event history.
// Snapshots are supported for performance optimization on large event streams.
package eventsourcing
