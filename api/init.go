package api

import (
	"fmt"
	"net"
	"net/http"

	"github.com/alibaba/sentinel-golang/core/config"
	"github.com/alibaba/sentinel-golang/core/log/metric"
	"github.com/alibaba/sentinel-golang/core/system_metric"
	metric_exporter "github.com/alibaba/sentinel-golang/exporter/metric"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/pkg/errors"
)

// Initialization func initialize the Sentinel's runtime environment, including:
//  1. override global config, from manually config or yaml file or env variable
//  2. override global logger
//  3. initiate core component async task, including: metric log, system statistic...
//
// InitDefault initializes Sentinel using the configuration from system
// environment and the default value.
func InitDefault() error {
	return initSentinel("")
}

// InitWithParser initializes Sentinel using given config parser
// parser deserializes the configBytes and return config.Entity
func InitWithParser(configBytes []byte, parser func([]byte) (*config.Entity, error)) (err error) {
	if parser == nil {
		return errors.New("nil parser")
	}
	confEntity, err := parser(configBytes)
	if err != nil {
		return err
	}
	return InitWithConfig(confEntity)
}

// InitWithConfig initializes Sentinel using given config.
func InitWithConfig(confEntity *config.Entity) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()

	err = config.CheckValid(confEntity)
	if err != nil {
		return err
	}
	config.ResetGlobalConfig(confEntity)
	if err = config.OverrideConfigFromEnvAndInitLog(); err != nil {
		return err
	}
	return initCoreComponents()
}

// Init loads Sentinel general configuration from the given YAML file
// and initializes Sentinel.
func InitWithConfigFile(configPath string) error {
	return initSentinel(configPath)
}

func initCoreComponents() error {
	if config.MetricLogFlushIntervalSec() > 0 {
		if err := metric.InitTask(); err != nil {
			return err
		}
	}

	systemStatInterval := config.SystemStatCollectIntervalMs()
	loadStatInterval := systemStatInterval
	cpuStatInterval := systemStatInterval
	memStatInterval := systemStatInterval

	if config.LoadStatCollectIntervalMs() > 0 {
		loadStatInterval = config.LoadStatCollectIntervalMs()
	}
	if config.CpuStatCollectIntervalMs() > 0 {
		cpuStatInterval = config.CpuStatCollectIntervalMs()
	}
	if config.MemoryStatCollectIntervalMs() > 0 {
		memStatInterval = config.MemoryStatCollectIntervalMs()
	}

	if loadStatInterval > 0 {
		system_metric.InitLoadCollector(loadStatInterval)
	}
	if cpuStatInterval > 0 {
		system_metric.InitCpuCollector(cpuStatInterval)
	}
	if memStatInterval > 0 {
		system_metric.InitMemoryCollector(memStatInterval)
	}

	if config.UseCacheTime() {
		util.StartTimeTicker()
	}

	if config.MetricExportHTTPAddr() != "" {
		httpAddr := config.MetricExportHTTPAddr()
		httpPath := config.MetricExportHTTPPath()

		l, err := net.Listen("tcp", httpAddr)
		if err != nil {
			return fmt.Errorf("init metric exporter http server err: %s", err.Error())
		}

		http.Handle(httpPath, metric_exporter.HTTPHandler())
		go func() {
			_ = http.Serve(l, nil)
		}()

		return nil
	}

	return nil
}

func initSentinel(configPath string) (err error) {
	defer func() {
		if r := recover(); r != nil {
			var ok bool
			err, ok = r.(error)
			if !ok {
				err = fmt.Errorf("%v", r)
			}
		}
	}()
	// Initialize general config and logging module.
	if err = config.InitConfigWithYaml(configPath); err != nil {
		return err
	}
	return initCoreComponents()
}
