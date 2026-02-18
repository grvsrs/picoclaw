package domain

// ---------------------------------------------------------------------------
// Repository pattern — persistence abstraction for all aggregates
// ---------------------------------------------------------------------------

// Repository defines the generic CRUD contract for aggregate persistence.
// Each bounded context provides a typed repository interface extending this.
type Repository[T any] interface {
	// FindByID retrieves an aggregate by its identity.
	FindByID(id EntityID) (*T, error)
	// Save persists an aggregate (create or update).
	Save(entity *T) error
	// Delete removes an aggregate by its identity.
	Delete(id EntityID) error
	// FindAll returns all aggregates.
	FindAll() ([]*T, error)
}

// ---------------------------------------------------------------------------
// Specification pattern — composable query predicates
// ---------------------------------------------------------------------------

// Specification defines a predicate for filtering domain objects.
// Used for queries without coupling domain logic to persistence.
type Specification[T any] interface {
	IsSatisfiedBy(entity *T) bool
}

// AndSpec combines two specifications with AND logic.
type AndSpec[T any] struct {
	Left  Specification[T]
	Right Specification[T]
}

func (s AndSpec[T]) IsSatisfiedBy(entity *T) bool {
	return s.Left.IsSatisfiedBy(entity) && s.Right.IsSatisfiedBy(entity)
}

// OrSpec combines two specifications with OR logic.
type OrSpec[T any] struct {
	Left  Specification[T]
	Right Specification[T]
}

func (s OrSpec[T]) IsSatisfiedBy(entity *T) bool {
	return s.Left.IsSatisfiedBy(entity) || s.Right.IsSatisfiedBy(entity)
}

// NotSpec negates a specification.
type NotSpec[T any] struct {
	Spec Specification[T]
}

func (s NotSpec[T]) IsSatisfiedBy(entity *T) bool {
	return !s.Spec.IsSatisfiedBy(entity)
}

// ---------------------------------------------------------------------------
// Unit of Work pattern — transactional boundary
// ---------------------------------------------------------------------------

// UnitOfWork coordinates persistence and event dispatch within a single
// business transaction. After Commit(), pending domain events are published.
type UnitOfWork interface {
	// Begin starts a new unit of work.
	Begin() error
	// Commit persists all changes and dispatches domain events.
	Commit() error
	// Rollback discards all changes.
	Rollback() error
	// RegisterNew marks an aggregate as newly created.
	RegisterNew(aggregate interface{})
	// RegisterDirty marks an aggregate as modified.
	RegisterDirty(aggregate interface{})
	// RegisterDeleted marks an aggregate for removal.
	RegisterDeleted(aggregate interface{})
}
