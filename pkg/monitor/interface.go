package monitor

type Discoverer interface {
	// Discover returns discovered data, warnings and error
	Discover(region, query string) (data map[string]interface{}, warns []error, e error)
}
