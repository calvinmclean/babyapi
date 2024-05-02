package kv

import (
	"fmt"
	"net/url"
	"time"
)

// EndDateable allows soft-delete by setting an end-date on resources instead of deleting them
type EndDateable interface {
	EndDated() bool
	SetEndDate(time.Time)
}

func EndDatedQueryParam(value bool) url.Values {
	return url.Values{"end_dated": []string{fmt.Sprint(value)}}
}
