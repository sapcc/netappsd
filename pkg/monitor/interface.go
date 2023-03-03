package monitor

type Discoverer interface {
	Discover(region, query string) (data map[string]interface{}, e error)
}
