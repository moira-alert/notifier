package notifier

// Settings is a collection of configuration options
type Settings interface {
	Get(section, key string) string
	GetInterface(section, key string) interface{}
}
