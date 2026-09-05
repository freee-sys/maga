package http

import (
	"fmt"
	"net/http"
	"strconv"
)

const (
	defaultPageLimit = 100
	maxPageLimit     = 1000
)

// parsePagination reads ?limit=&offset= from a request, applying the
// default/max limit and rejecting negative or non-numeric values.
func parsePagination(r *http.Request) (limit, offset int, err error) {
	limit = defaultPageLimit
	if raw := r.URL.Query().Get("limit"); raw != "" {
		limit, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, fmt.Errorf("limit must be an integer")
		}
		if limit < 0 {
			return 0, 0, fmt.Errorf("limit must not be negative")
		}
		if limit > maxPageLimit {
			limit = maxPageLimit
		}
	}

	if raw := r.URL.Query().Get("offset"); raw != "" {
		offset, err = strconv.Atoi(raw)
		if err != nil {
			return 0, 0, fmt.Errorf("offset must be an integer")
		}
		if offset < 0 {
			return 0, 0, fmt.Errorf("offset must not be negative")
		}
	}

	return limit, offset, nil
}
