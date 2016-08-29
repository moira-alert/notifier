package notifier

type Config struct {
	Redis    RedisConfig    `yaml:"redis"`
	Front    FrontConfig    `yaml:"front"`
	Graphite GraphiteConfig `yaml:"graphite"`
	Notifier NotifierConfig `yaml:"notifier"`
}

type NotifierConfig struct {
	LogFile          string              `yaml:"log_file"`
	LogLevel         string              `yaml:"log_level"`
	LogColor         string              `yaml:"log_color"`
	SenderTimeout    string              `yaml:"sender_timeout"`
	ResendingTimeout string              `yaml:"resending_timeout"`
	Senders          []map[string]string `yaml:"senders"`
	SelfState        SelfStateConfig     `yaml:"moira_selfstate"`
}

type RedisConfig struct {
	Host string `yaml:"host"`
	Port string `yaml:"port"`
	DBID int    `yaml:"dbid"`
}

type FrontConfig struct {
	URI string `yaml:"uri"`
}

type GraphiteConfig struct {
	URI      string `yaml:"uri"`
	Prefix   string `yaml:"prefix"`
	Interval int64  `yaml:"interval"`
}

type SelfStateConfig struct {
	Enabled                 string              `yaml:"enabled"`
	RedisDisconectDelay     int64               `yaml:"redis_disconect_delay"`
	LastMetricReceivedDelay int64               `yaml:"last_metric_received_delay"`
	LastCheckDelay          int64               `yaml:"last_check_delay"`
	Contacts                []map[string]string `yaml:"contacts"`
	NoticeInterval          int64               `yaml:"notice_interval"`
}
