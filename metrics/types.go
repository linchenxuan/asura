package metrics

// Policy defines the aggregation policy for metric values.
// It determines how multiple values for the same metric should be combined over time.
type Policy int

const (
	PolicyNone      Policy = iota // No specific policy specified
	PolicySet                     // Instantaneous value - last value wins
	PolicySum                     // Sum of all values
	PolicyAvg                     // Average of all values
	PolicyMax                     // Maximum value
	PolicyMin                     // Minimum value
	PolicyMid                     // Median value
	PolicyStopwatch               // Timer - measures duration
	PolicyHistogram               // Histogram statistics
)

// Value represents a metric value as a float64.
type Value float64

// Dimension represents metric dimensions as key-value pairs.
// Dimensions are used to add contextual information to metrics,
// such as server name, region, version, etc.
type Dimension map[string]string
