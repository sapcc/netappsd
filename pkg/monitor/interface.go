package monitor

type Watcher interface {
	Observe(promQ, labelName string) (names []string, e error)
	Discover(region, query string) (data map[string]interface{}, e error)
}
