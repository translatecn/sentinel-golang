package config

import (
	"encoding/json"
	"fmt"

	"github.com/alibaba/sentinel-golang/core/base"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/pkg/errors"
)

type Entity struct {
	Version  string // 表示实体的格式版本.
	Sentinel SentinelConfig
}

// SentinelConfig 代表哨兵的一般结构.
type SentinelConfig struct {
	App struct {
		Name string // 代表哨兵的一般结构.表示当前正在运行的服务的名称.
		Type int32  // 表示服务的分类(如web服务，API网关).
	}
	Exporter     ExporterConfig // 表示与导出器相关的配置项，如度量导出器.
	Log          LogConfig      //
	Stat         StatConfig     //
	UseCacheTime bool           `yaml:"useCacheTime"` // 是否缓存时间(毫秒)
}

type ExporterConfig struct {
	Metric MetricExporterConfig
}

type MetricExporterConfig struct {
	HttpAddr string `yaml:"http_addr"` // HTTP服务器监听地址，如“:8080”.
	HttpPath string `yaml:"http_path"` // 是访问度量的HTTP请求路径，如“/metrics”.
}

type LogConfig struct {
	Logger logging.Logger  //
	Dir    string          //
	UsePid bool            `yaml:"usePid"` // 表示文件名是否以进程ID (PID)结束.
	Metric MetricLogConfig // 表示度量日志的配置项.
}

type MetricLogConfig struct {
	SingleFileMaxSize uint64 `yaml:"singleFileMaxSize"`
	MaxFileCount      uint32 `yaml:"maxFileCount"`
	FlushIntervalSec  uint32 `yaml:"flushIntervalSec"`
}

// StatConfig 表示统计信息的配置项.
type StatConfig struct {
	// 是每个资源的全局默认统计滑动窗口配置
	GlobalStatisticSampleCountTotal uint32 `yaml:"globalStatisticSampleCountTotal"`
	GlobalStatisticIntervalMsTotal  uint32 `yaml:"globalStatisticIntervalMsTotal"`

	// 每个资源的默认只读指标统计吗
	// 这个默认的只读度量统计必须基于全局统计可重用.
	MetricStatisticSampleCount uint32           `yaml:"metricStatisticSampleCount"`
	MetricStatisticIntervalMs  uint32           `yaml:"metricStatisticIntervalMs"`
	System                     SystemStatConfig `yaml:"system"`
}

type SystemStatConfig struct {
	CollectIntervalMs       uint32 `yaml:"collectIntervalMs"`       // 表示系统指标收集器的收集间隔.
	CollectLoadIntervalMs   uint32 `yaml:"collectLoadIntervalMs"`   // 表示系统负载收集器的收集间隔.
	CollectCpuIntervalMs    uint32 `yaml:"collectCpuIntervalMs"`    // 表示系统CPU使用率收集器的收集间隔.
	CollectMemoryIntervalMs uint32 `yaml:"collectMemoryIntervalMs"` // 表示系统内存使用收集器的收集间隔.
}

func NewDefaultConfig() *Entity {
	return &Entity{
		Version: "v1",
		Sentinel: SentinelConfig{
			App: struct {
				Name string
				Type int32
			}{
				Name: UnknownProjectName,
				Type: DefaultAppType,
			},
			Log: LogConfig{
				Logger: nil,
				Dir:    GetDefaultLogDir(),
				UsePid: false,
				Metric: MetricLogConfig{
					SingleFileMaxSize: DefaultMetricLogSingleFileMaxSize,
					MaxFileCount:      DefaultMetricLogMaxFileAmount,
					FlushIntervalSec:  DefaultMetricLogFlushIntervalSec,
				},
			},
			Stat: StatConfig{
				GlobalStatisticSampleCountTotal: base.DefaultSampleCountTotal,
				GlobalStatisticIntervalMsTotal:  base.DefaultIntervalMsTotal,
				MetricStatisticSampleCount:      base.DefaultSampleCount,
				MetricStatisticIntervalMs:       base.DefaultIntervalMs,
				System: SystemStatConfig{
					CollectIntervalMs:       DefaultSystemStatCollectIntervalMs,
					CollectLoadIntervalMs:   DefaultLoadStatCollectIntervalMs,
					CollectCpuIntervalMs:    DefaultCpuStatCollectIntervalMs,
					CollectMemoryIntervalMs: DefaultMemoryStatCollectIntervalMs,
				},
			},
			UseCacheTime: false,
		},
	}
}

func CheckValid(entity *Entity) error {
	if entity == nil {
		return errors.New("Nil entity")
	}
	if len(entity.Version) == 0 {
		return errors.New("Empty version")
	}
	return checkConfValid(&entity.Sentinel)
}

func checkConfValid(conf *SentinelConfig) error {
	if conf == nil {
		return errors.New("Nil globalCfg")
	}
	if conf.App.Name == "" {
		return errors.New("App.Name is empty")
	}
	mc := conf.Log.Metric
	if mc.MaxFileCount <= 0 {
		return errors.New("Illegal metric log globalCfg: maxFileCount <= 0")
	}
	if mc.SingleFileMaxSize <= 0 {
		return errors.New("Illegal metric log globalCfg: singleFileMaxSize <= 0")
	}
	if err := base.CheckValidityForReuseStatistic(conf.Stat.MetricStatisticSampleCount, conf.Stat.MetricStatisticIntervalMs,
		conf.Stat.GlobalStatisticSampleCountTotal, conf.Stat.GlobalStatisticIntervalMsTotal); err != nil {
		return err
	}
	return nil
}

func (entity *Entity) String() string {
	e, err := json.Marshal(entity)
	if err != nil {
		return fmt.Sprintf("%+v", *entity)
	}
	return string(e)
}

func (entity *Entity) AppName() string {
	return entity.Sentinel.App.Name
}

func (entity *Entity) AppType() int32 {
	return entity.Sentinel.App.Type
}

func (entity *Entity) LogBaseDir() string {
	return entity.Sentinel.Log.Dir
}

func (entity *Entity) Logger() logging.Logger {
	return entity.Sentinel.Log.Logger
}

// LogUsePid returns whether the log file name contains the PID suffix.
func (entity *Entity) LogUsePid() bool {
	return entity.Sentinel.Log.UsePid
}

func (entity *Entity) MetricExportHTTPAddr() string {
	return entity.Sentinel.Exporter.Metric.HttpAddr
}

func (entity *Entity) MetricExportHTTPPath() string {
	return entity.Sentinel.Exporter.Metric.HttpPath
}

func (entity *Entity) MetricLogFlushIntervalSec() uint32 {
	return entity.Sentinel.Log.Metric.FlushIntervalSec
}

func (entity *Entity) MetricLogSingleFileMaxSize() uint64 {
	return entity.Sentinel.Log.Metric.SingleFileMaxSize
}

func (entity *Entity) MetricLogMaxFileAmount() uint32 {
	return entity.Sentinel.Log.Metric.MaxFileCount
}

func (entity *Entity) SystemStatCollectIntervalMs() uint32 {
	return entity.Sentinel.Stat.System.CollectIntervalMs
}

func (entity *Entity) LoadStatCollectIntervalMs() uint32 {
	return entity.Sentinel.Stat.System.CollectLoadIntervalMs
}

func (entity *Entity) CpuStatCollectIntervalMs() uint32 {
	return entity.Sentinel.Stat.System.CollectCpuIntervalMs
}

func (entity *Entity) MemoryStatCollectIntervalMs() uint32 {
	return entity.Sentinel.Stat.System.CollectMemoryIntervalMs
}

func (entity *Entity) UseCacheTime() bool {
	return entity.Sentinel.UseCacheTime
}

func (entity *Entity) GlobalStatisticIntervalMsTotal() uint32 {
	return entity.Sentinel.Stat.GlobalStatisticIntervalMsTotal
}

func (entity *Entity) GlobalStatisticSampleCountTotal() uint32 {
	return entity.Sentinel.Stat.GlobalStatisticSampleCountTotal
}

func (entity *Entity) MetricStatisticIntervalMs() uint32 {
	return entity.Sentinel.Stat.MetricStatisticIntervalMs
}
func (entity *Entity) MetricStatisticSampleCount() uint32 {
	return entity.Sentinel.Stat.MetricStatisticSampleCount
}
