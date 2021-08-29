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
}

func NewMetricsManager(metricsDirectory string) *MetricsManager {
	mgr := new(MetricsManager)
	os.MkdirAll(metricsDirectory, os.ModePerm)
	metricsIdentifier := time.Now().Format("02-01-2006-15h04m05s")
	bootFileName := fmt.Sprintf("bootmetrics%s", metricsIdentifier)
	createSnapFileName := fmt.Sprintf("createsnapmetrics%s", metricsIdentifier)

	// Create files
	mgr.bootFile, _ = os.Create(filepath.Join(metricsDirectory, bootFileName))
	mgr.createSnapFile, _ = os.Create(filepath.Join(metricsDirectory, createSnapFileName))

	// Create CSV writers
	mgr.bootWriter = csv.NewWriter(mgr.bootFile)
	mgr.createSnapWriter = csv.NewWriter(mgr.createSnapFile)

	// Write headers
	mgr.bootWriter.Write(GetBootHeaderLine())
	mgr.createSnapWriter.Write(GetSnapHeaderLine())

	return mgr
}

func (mgr *MetricsManager) Cleanup() {
	mgr.bootFile.Close()
	mgr.createSnapFile.Close()
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