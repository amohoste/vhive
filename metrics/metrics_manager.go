package metrics

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type MetricsManager struct {
	bootMutex         sync.Mutex
	bootFile          *os.File
	bootWriter        *csv.Writer

	createSnapMutex   sync.Mutex
	createSnapFile    *os.File
	createSnapWriter  *csv.Writer

	netMutex   sync.Mutex
	netFile    *os.File
	netWriter  *csv.Writer

	forkMutex   sync.Mutex
	forkFile    *os.File
	forkWriter  *csv.Writer
}

func NewMetricsManager(metricsDirectory string) *MetricsManager {
	mgr := new(MetricsManager)
	os.MkdirAll(metricsDirectory, os.ModePerm)
	metricsIdentifier := time.Now().Format("02-01-2006-15h04m05s")
	bootFileName := fmt.Sprintf("bootmetrics%s", metricsIdentifier)
	createSnapFileName := fmt.Sprintf("createsnapmetrics%s", metricsIdentifier)
	netFileName := fmt.Sprintf("netmetrics%s", metricsIdentifier)
	forkFileName := fmt.Sprintf("forkmetrics%s", metricsIdentifier)

	// Create files
	mgr.bootFile, _ = os.Create(filepath.Join(metricsDirectory, bootFileName))
	mgr.createSnapFile, _ = os.Create(filepath.Join(metricsDirectory, createSnapFileName))
	mgr.netFile, _ = os.Create(filepath.Join(metricsDirectory, netFileName))
	mgr.forkFile, _ = os.Create(filepath.Join(metricsDirectory, forkFileName))

	// Create CSV writers
	mgr.bootWriter = csv.NewWriter(mgr.bootFile)
	mgr.createSnapWriter = csv.NewWriter(mgr.createSnapFile)
	mgr.netWriter = csv.NewWriter(mgr.netFile)
	mgr.forkWriter = csv.NewWriter(mgr.forkFile)

	// Write headers
	mgr.bootWriter.Write(GetBootHeaderLine())
	mgr.bootWriter.Flush()

	mgr.createSnapWriter.Write(GetSnapHeaderLine())
	mgr.createSnapWriter.Flush()

	mgr.netWriter.Write(GetNetHeaderLine())
	mgr.netWriter.Flush()

	mgr.forkWriter.Write(GetForkHeaderLine())
	mgr.forkWriter.Flush()

	return mgr
}

func (mgr *MetricsManager) Cleanup() {
	mgr.bootFile.Close()
	mgr.createSnapFile.Close()
	mgr.netFile.Close()
}

func (mgr *MetricsManager) AddBootMetric(metric *BootMetric) error {
	mgr.bootMutex.Lock()
	defer mgr.bootMutex.Unlock()

	if err := mgr.bootWriter.Write(metric.GetValueLine()); err != nil {
		return err
	}
	mgr.bootWriter.Flush()
	return nil
}

func (mgr *MetricsManager) AddSnapMetric(metric *SnapMetric) error {
	mgr.createSnapMutex.Lock()
	defer mgr.createSnapMutex.Unlock()
	if err := mgr.createSnapWriter.Write(metric.GetValueLine()); err != nil {
		return err
	}
	mgr.createSnapWriter.Flush()
	return nil
}

func (mgr *MetricsManager) AddNetMetric(metric *NetMetric) error {
	mgr.netMutex.Lock()
	defer mgr.netMutex.Unlock()
	if err := mgr.netWriter.Write(metric.GetValueLine()); err != nil {
		return err
	}
	mgr.netWriter.Flush()
	return nil
}

func (mgr *MetricsManager) AddForkMetric(metric *ForkMetric) error {
	mgr.forkMutex.Lock()
	defer mgr.forkMutex.Unlock()
	if err := mgr.forkWriter.Write(metric.GetValueLine()); err != nil {
		return err
	}
	mgr.forkWriter.Flush()
	return nil
}