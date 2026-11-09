package sqlite

import "kutubcore/internal/domain/id"

func uuidGen() string { return id.Must() }
