package notifier

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"

	"github.com/cyberdelia/go-metrics-graphite"
	"github.com/rcrowley/go-metrics"
)

// InitMetrics inits graphite metrics and starts graphite flush cycle
func InitMetrics() {
	graphiteURI := config.Graphite.URI
	graphitePrefix := config.Graphite.Prefix
	graphiteInterval := config.Graphite.Interval

	if graphiteURI != "" {
		graphiteAddr, err := net.ResolveTCPAddr("tcp", graphiteURI)
		if err != nil {
			log.Errorf("Can not resolve graphiteURI %s: %s", graphiteURI, err)
			return
		}
		hostname, err := os.Hostname()
		if err != nil {
			log.Errorf("Can not get OS hostname: %s", err)
			return
		}
		shortname := strings.Split(hostname, ".")[0]
		go graphite.Graphite(metrics.DefaultRegistry, time.Duration(graphiteInterval)*time.Second, fmt.Sprintf("%s.notifier.%s", graphitePrefix, shortname), graphiteAddr)
	}
}
