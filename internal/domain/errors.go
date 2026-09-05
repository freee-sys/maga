package domain

import "errors"

// ErrNotFound is returned by repositories when a lookup by ID finds no
// row, so callers (typically HTTP handlers) can map it to 404 without
// depending on a specific storage driver's error type.
var ErrNotFound = errors.New("not found")

// ErrClusterNotEmpty is returned by ClusterRepo.Delete when the cluster
// still has member elements, so callers can map it to 409 without
// depending on a specific storage driver's constraint-violation type.
var ErrClusterNotEmpty = errors.New("cluster is not empty")
