package notifier

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cyberdelia/go-metrics-graphite"
	"github.com/gosexy/to"
	"github.com/rcrowley/go-metrics"
)

// InitMetrics inits graphite metrics and starts graphite flush cycle
func InitMetrics() {
	graphiteURI := config.Get("graphite", "uri")
	graphitePrefix := config.Get("graphite", "prefix")
	graphiteInterval := to.Int64(config.Get("graphite", "interval"))

	if graphiteURI != "" {
		graphiteAddr, err := net.ResolveTCPAddr("tcp", graphiteURI)
		if err != nil {
			log.Error("Can not resolve graphiteURI %s: %s", graphiteURI, err.Error())
			return
		}
		hostname, err := os.Hostname()
		if err != nil {
			log.Error("Can not get OS hostname %s", err.Error())
			return
		}
		shortname := strings.Split(hostname, ".")[0]
		go graphite.Graphite(metrics.DefaultRegistry, time.Duration(graphiteInterval)*time.Second, fmt.Sprintf("%s.notifier.%s", graphitePrefix, shortname), graphiteAddr)
	}
}
