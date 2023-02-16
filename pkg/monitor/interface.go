package monitor

type Monitor interface {
	Observe(promQ, labelName string) (names []string, e error)
	Discover(region, query string) (data map[string]interface{}, e error)
}
