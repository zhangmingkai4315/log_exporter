package collector

import (
	"context"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/hpcloud/tail"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/common/log"
	"github.com/vjeantet/grok"
	"github.com/zhangmingkai4315/log_exporter/config"
)

const (
	namespace = "log_exporter"
)

// LogManager will store log information
type LogManager struct {
	mutex  sync.RWMutex
	ctx    context.Context
	cancel context.CancelFunc

	fileTail   *tail.Tail
	fileConfig config.FileConfig
	g          *grok.Grok
	rawData    []map[string]map[string]uint64
}

// NewLogManager return a new LogManager for each file
func NewLogManager(ctx context.Context, fc config.FileConfig, g *grok.Grok) (*LogManager, error) {
	var location *tail.SeekInfo
	if fc.Readall == false {
		location = &tail.SeekInfo{Offset: 0, Whence: os.SEEK_END}
	} else {
		location = &tail.SeekInfo{Offset: 0, Whence: os.SEEK_SET}
	}
	tailFile, err := tail.TailFile(fc.Path, tail.Config{Follow: true, ReOpen: true, Location: location})
	if err != nil {
		return nil, err
	}
	newCtx, cancel := context.WithCancel(ctx)
	return &LogManager{
		fileConfig: fc,
		fileTail:   tailFile,
		g:          g,
		ctx:        newCtx,
		cancel:     cancel,
	}, nil
}

// Start will start goroutine to parse logs
func (logManager *LogManager) Start() {
	log.Infof("Start log parser for file:%+v", logManager.fileConfig)
	logManager.rawData = make([]map[string]map[string]uint64, logManager.fileConfig.Worker)
	pattern := logManager.fileConfig.Metric.Match
	for i := 0; i < logManager.fileConfig.Worker; i++ {
		go func(index int, parentContext context.Context) {
			log.Infof("Start worker %d for file:%v", index+1, logManager.fileConfig.Path)
			ctx, cancel := context.WithCancel(parentContext)
			defer cancel()
			logManager.rawData[index] = make(map[string]map[string]uint64)
			rawData := logManager.rawData[index]
			reversLabel := make(map[string]string)
			for newLabel, oldLabel := range logManager.fileConfig.Metric.Labels {
				rawData[newLabel] = make(map[string]uint64)
				reversLabel[oldLabel] = newLabel
			}
			for {
				log.Infoln("...")
				select {
				case <-ctx.Done():
					log.Infoln("Recive exit signal from main goroutine")
					return
				case line := <-logManager.fileTail.Lines:
					rawInfo, err := logManager.g.Parse(pattern, line.Text)
					if err != nil {
						log.Errorf("Error parser for file:%v error:%v", logManager.fileConfig.Path, err)
						continue
					}
					for k, v := range rawInfo {
						if newLable, ok := reversLabel[k]; ok {
							// should using atomic to save update
							if _, ok := rawData[newLable][v]; ok {
								rawData[newLable][v] = rawData[newLable][v] + 1
							} else {
								rawData[newLable][v] = 1
							}
						}
					}

				}
			}
		}(i, logManager.ctx)
	}

}

// Start will start goroutine to parse logs
func (logManager *LogManager) collect(ch chan<- prometheus.Metric, lgs *LogManagers) error {

	logManager.mutex.Lock()
	defer logManager.mutex.Unlock()
	temp := logManager.rawData[0]
	// for i, r := range logManager.rawData {
	// 	if i == 0 {
	// 		continue
	// 	}
	// 	for k, v := range r {
	// 		if tv, ok := temp[k]; ok {
	// 			for vk, vv := range v {
	// 				if tvk, ok := temp[k][vk]; ok {
	// 					temp[k][vk] = temp[k][vk] + vv
	// 				} else {
	// 					temp[k][vk] = vv
	// 				}
	// 			}
	// 		} else {
	// 			temp[k] = v
	// 		}
	// 	}
	// }

	ch <- prometheus.MustNewConstMetric(lgs.totalLogCounter, prometheus.CounterValue, float64(20), logManager.fileConfig.Path, "acv")
	ch <- prometheus.MustNewConstMetric(lgs.totalLogCounter, prometheus.CounterValue, float64(10), logManager.fileConfig.Path, "www")
	return nil
}

// LogManagers will collect a sort of log manager
type LogManagers struct {
	mutex           sync.RWMutex
	ctx             context.Context
	grokObject      *grok.Grok
	logManages      []*LogManager
	currentTailFile prometheus.Gauge
	totalLogCounter *prometheus.Desc
	processFailures prometheus.Counter
	freezeTime      *prometheus.Desc
}

// Describe implements prometheus.Collector.
func (lgs *LogManagers) Describe(ch chan<- *prometheus.Desc) {
	ch <- lgs.totalLogCounter
	lgs.currentTailFile.Describe(ch)
	lgs.processFailures.Describe(ch)
}

// Collect implements prometheus.Collector.
func (lgs *LogManagers) Collect(ch chan<- prometheus.Metric) {
	lgs.mutex.Lock()
	defer lgs.mutex.Unlock()
	begin := time.Now()
	lgs.currentTailFile.Set(0)
	wg := sync.WaitGroup{}
	for _, logManager := range lgs.logManages {
		wg.Add(1)
		go func(logManager *LogManager) {
			defer wg.Done()
			if err := logManager.collect(ch, lgs); err != nil {
				log.Errorf("Error collect data from : %v", logManager.fileConfig.Path)
				log.Errorf("Error message: %v", err)
				lgs.processFailures.Inc()
			} else {
				lgs.currentTailFile.Inc()
			}

		}(logManager)
		wg.Wait()
	}
	duration := time.Since(begin)
	ch <- prometheus.MustNewConstMetric(lgs.freezeTime, prometheus.GaugeValue, float64(duration.Nanoseconds()), "file_scrape")

	lgs.processFailures.Collect(ch)
	lgs.currentTailFile.Collect(ch)
	return
}

// NewLogManagers will create a new log managers
func NewLogManagers(ctx context.Context, cfg *config.Config) (*LogManagers, error) {
	currentTailFile := prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: namespace,
		Name:      "open_files_number",
		Help:      "files number for parser process",
	})
	freezeTime := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "freeze_time", "collector_duration_seconds"),
		"log_exporter: Duration of a collector freeze.",
		[]string{"action"},
		nil,
	)
	totalLogCounter := prometheus.NewDesc(
		prometheus.BuildFQName(namespace, "", "total"),
		"Counter of log labels",
		[]string{"file", "name"},
		nil,
	)
	processFailures := prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: namespace,
		Name:      "failure_total",
		Help:      "Number of failures while grok the log file",
	})
	g, _ := grok.New()
	g.AddPatternsFromPath(cfg.Global.GrokDir)
	for _, f := range cfg.Files {

		for _, grokString := range f.Customgroks {
			t := strings.Split(grokString, " ")
			if len(t) < 2 {
				continue
			}
			g.AddPattern(t[0], t[1])
		}
	}

	logManagers := &LogManagers{
		currentTailFile: currentTailFile,
		totalLogCounter: totalLogCounter,
		processFailures: processFailures,
		freezeTime:      freezeTime,
		ctx:             ctx,
		grokObject:      g,
	}
	for _, file := range cfg.Files {
		logManager, err := NewLogManager(ctx, file, g)
		if err != nil {
			return nil, err
		}
		go logManager.Start()
		logManagers.logManages = append(logManagers.logManages, logManager)
		log.Infof("%v", logManager.fileConfig.Path)
	}
	return logManagers, nil
}
