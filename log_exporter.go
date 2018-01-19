package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/prometheus/common/version"
	"github.com/zhangmingkai4315/log_exporter/collector"
	"github.com/zhangmingkai4315/log_exporter/config"
	"gopkg.in/alecthomas/kingpin.v2"
)

func init() {
	prometheus.MustRegister(version.NewCollector("log_exporter"))
}
func main() {
	var (
		configFile = kingpin.Flag("config", "").Short('c').Default("config.yml").String()
	)
	log.AddFlags(kingpin.CommandLine)
	kingpin.Version(version.Print("log_exporter"))
	kingpin.HelpFlag.Short('h')
	kingpin.Parse()
	log.Infoln("Starting log_exporter", version.Info())
	log.Infoln("Build context", version.BuildContext())

	config, err := config.LoadConfig(*configFile)
	if err != nil {
		log.Errorf("Config validation fail:%s", err.Error())
		os.Exit(1)
	}
	log.Infoln("Config validation success")
	for _, file := range config.Files {
		log.Infof("Config file path: %v", file.Path)
	}
	ctx, cancel := context.WithCancel(context.Background())
	logManagers, err := collector.NewLogManagers(ctx, config)
	if err != nil {
		log.Errorf("Log managers create fail:%s", err.Error())
		os.Exit(1)
	}
	stopSignal := make(chan os.Signal, 1)
	signal.Notify(stopSignal, os.Interrupt)
	go func() {
		for _ = range stopSignal {
			log.Infoln("Receive main quit signal, send broadcast signal to works")
			cancel()
			os.Exit(1)
		}
	}()

	prometheus.MustRegister(logManagers)
	http.Handle(config.Global.MetricsPath, prometheus.Handler())
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Write([]byte(`<html>
			<head><title>Node Exporter</title></head>
			<body>
			<h1>Log Exporter</h1>
			<p><a href="` + config.Global.MetricsPath + `">Metrics</a></p>
			</body>
			</html>`))
	})
	serverInfo := config.Global.Server.ServerListenInfo()
	log.Infoln("HTTP server listen at " + serverInfo)
	log.Fatalln(http.ListenAndServe(serverInfo, nil))
}
