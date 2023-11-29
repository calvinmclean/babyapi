package storage

import (
	"time"
)

// EndDateable allows soft-delete by setting an end-date on resources instead of deleting them
type EndDateable interface {
	EndDated() bool
	SetEndDate(time.Time)
}
