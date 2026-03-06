// Package cqrs implements the Command Query Responsibility Segregation pattern.
//
// Commands represent write operations and are dispatched through [CommandBus]
// to registered [CommandHandler] implementations. Queries represent read
// operations and are handled by [QueryHandler] implementations.
// The CommandBus supports middleware for cross-cutting concerns like logging
// and validation.
package cqrs
