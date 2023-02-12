package system_metric

import (
	"os"
	"sync"
	"sync/atomic"
	"time"

	metric_exporter "github.com/alibaba/sentinel-golang/exporter/metric"
	"github.com/alibaba/sentinel-golang/logging"
	"github.com/alibaba/sentinel-golang/util"
	"github.com/shirou/gopsutil/v3/load"
	"github.com/shirou/gopsutil/v3/mem"
	"github.com/shirou/gopsutil/v3/process"
)

const (
	NotRetrievedLoadValue     float64 = -1.0
	NotRetrievedCpuUsageValue float64 = -1.0
	NotRetrievedMemoryValue   int64   = -1
)

var (
	currentLoad        atomic.Value
	currentCpuUsage    atomic.Value
	currentMemoryUsage atomic.Value // InitMemoryCollector 函数定时更新

	loadStatCollectorOnce   sync.Once
	memoryStatCollectorOnce sync.Once
	cpuStatCollectorOnce    sync.Once

	CurrentPID         = os.Getpid()
	currentProcess     atomic.Value
	currentProcessOnce sync.Once
	TotalMemorySize    = getTotalMemorySize()

	ssStopChan = make(chan struct{})

	cpuRatioGauge = metric_exporter.NewGauge(
		"cpu_ratio",
		"Process cpu ratio",
		[]string{})
	processMemoryGauge = metric_exporter.NewGauge(
		"process_memory_bytes",
		"Process memory in bytes",
		[]string{})
)

func init() {
	currentLoad.Store(NotRetrievedLoadValue)
	currentCpuUsage.Store(NotRetrievedCpuUsageValue)
	currentMemoryUsage.Store(NotRetrievedMemoryValue)

	p, err := process.NewProcess(int32(CurrentPID))
	if err != nil {
		logging.Error(err, "Fail to new process when initializing system metric", "pid", CurrentPID)
		return
	}
	currentProcessOnce.Do(func() {
		currentProcess.Store(p)
	})

	metric_exporter.Register(cpuRatioGauge)
	metric_exporter.Register(processMemoryGauge)
}

// getMemoryStat returns the current machine's memory statistic
func getTotalMemorySize() (total uint64) {
	stat, err := mem.VirtualMemory()
	if err != nil {
		logging.Error(err, "Fail to read Virtual Memory")
		return 0
	}
	return stat.Total
}

func InitMemoryCollector(intervalMs uint32) {
	if intervalMs == 0 {
		return
	}
	memoryStatCollectorOnce.Do(func() {
		// Initial memory retrieval.
		retrieveAndUpdateMemoryStat()

		ticker := util.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		go util.RunWithRecover(func() {
			for {
				select {
				case <-ticker.C():
					retrieveAndUpdateMemoryStat() // 函数定时更新
				case <-ssStopChan:
					ticker.Stop()
					return
				}
			}
		})
	})
}

func retrieveAndUpdateMemoryStat() { // 函数定时更新
	memoryUsedBytes, err := GetProcessMemoryStat()
	if err != nil {
		logging.Error(err, "Fail to retrieve and update memory statistic")
		return
	}
	processMemoryGauge.Set(float64(memoryUsedBytes)) // 上报指标
	currentMemoryUsage.Store(memoryUsedBytes)        // 函数定时更新
}

// GetProcessMemoryStat gets current process's memory usage in Bytes
func GetProcessMemoryStat() (int64, error) {
	curProcess := currentProcess.Load()
	if curProcess == nil {
		p, err := process.NewProcess(int32(CurrentPID))
		if err != nil {
			return 0, err
		}
		currentProcessOnce.Do(func() {
			currentProcess.Store(p)
		})
		curProcess = currentProcess.Load()
	}
	p := curProcess.(*process.Process)
	memInfo, err := p.MemoryInfo()
	var rss int64
	if memInfo != nil {
		rss = int64(memInfo.RSS)
	}

	return rss, err
}

func InitCpuCollector(intervalMs uint32) {
	if intervalMs == 0 {
		return
	}
	cpuStatCollectorOnce.Do(func() {
		// Initial memory retrieval.
		retrieveAndUpdateCpuStat()

		ticker := util.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		go util.RunWithRecover(func() {
			for {
				select {
				case <-ticker.C():
					retrieveAndUpdateCpuStat()
				case <-ssStopChan:
					ticker.Stop()
					return
				}
			}
		})
	})
}

func retrieveAndUpdateCpuStat() {
	cpuPercent, err := getProcessCpuStat()
	if err != nil {
		logging.Error(err, "Fail to retrieve and update cpu statistic")
		return
	}

	cpuRatioGauge.Set(cpuPercent) // 上报指标

	currentCpuUsage.Store(cpuPercent)
}

// getProcessCpuStat gets current process's memory usage in Bytes
func getProcessCpuStat() (float64, error) {
	curProcess := currentProcess.Load()
	if curProcess == nil {
		p, err := process.NewProcess(int32(CurrentPID))
		if err != nil {
			return 0, err
		}
		currentProcessOnce.Do(func() {
			currentProcess.Store(p)
		})
		curProcess = currentProcess.Load()
	}
	p := curProcess.(*process.Process)
	return p.Percent(0)
}

func InitLoadCollector(intervalMs uint32) {
	if intervalMs == 0 {
		return
	}
	loadStatCollectorOnce.Do(func() {
		// Initial retrieval.
		retrieveAndUpdateLoadStat()

		ticker := util.NewTicker(time.Duration(intervalMs) * time.Millisecond)
		go util.RunWithRecover(func() {
			for {
				select {
				case <-ticker.C():
					retrieveAndUpdateLoadStat()
				case <-ssStopChan:
					ticker.Stop()
					return
				}
			}
		})
	})
}

func retrieveAndUpdateLoadStat() {
	loadStat, err := load.Avg()
	if err != nil {
		logging.Error(err, "[retrieveAndUpdateSystemStat] Failed to retrieve current system load")
		return
	}
	if loadStat != nil {
		currentLoad.Store(loadStat.Load1)
	}
}

func CurrentLoad() float64 {
	r, ok := currentLoad.Load().(float64)
	if !ok {
		return NotRetrievedLoadValue
	}
	return r
}

// SetSystemLoad 用于单元测试，用户不应该调用此函数.
func SetSystemLoad(load float64) {
	currentLoad.Store(load)
}

func CurrentCpuUsage() float64 {
	r, ok := currentCpuUsage.Load().(float64)
	if !ok {
		return NotRetrievedCpuUsageValue
	}
	return r
}

// SetSystemCpuUsage 用于单元测试，用户不应该调用此函数.
func SetSystemCpuUsage(cpuUsage float64) {
	currentCpuUsage.Store(cpuUsage)
}

func CurrentMemoryUsage() int64 {
	bytes, ok := currentMemoryUsage.Load().(int64)
	if !ok {
		return NotRetrievedMemoryValue
	}

	return bytes
}

// SetSystemMemoryUsage 用于单元测试，用户不应该调用此函数.
func SetSystemMemoryUsage(memoryUsage int64) {
	currentMemoryUsage.Store(memoryUsage)
}
