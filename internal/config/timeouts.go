package config

import (
	"time"
)

// DataReadTimeout is the maximum allowed time for a data query to take before it is cancelled
const DataReadTimeout = time.Duration(15 * time.Minute)

// DataWriteTimeout is the maximum allowed time for a data insertion to take before it is cancelled
const DataWriteTimeout = time.Duration(30 * time.Minute)
