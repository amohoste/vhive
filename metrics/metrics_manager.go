package metrics

import (
	"encoding/csv"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// MetricsManager manages writing metrics of multiple concurrent VMs to a CSV file.
type MetricsManager struct {
	bootMutex         sync.Mutex
	bootFile          *os.File
	bootWriter        *csv.Writer
}

// NewMetricsManager creates a new metric manager.
func NewMetricsManager(metricsDirectory string) *MetricsManager {
	mgr := new(MetricsManager)

	// Create metrics directory if it does not exist yet.
	os.MkdirAll(metricsDirectory, os.ModePerm)
	metricsIdentifier := time.Now().Format("02-01-2006-15h04m05s")

	// Create a name for the file where the boot latency metrics will be stored
	bootFileName := fmt.Sprintf("bootmetrics%s", metricsIdentifier)

	// Create metric files
	mgr.bootFile, _ = os.Create(filepath.Join(metricsDirectory, bootFileName))

	// Create CSV writers
	mgr.bootWriter = csv.NewWriter(mgr.bootFile)

	// Write CSV headers
	mgr.bootWriter.Write(GetBootHeaderLine())
	mgr.bootWriter.Flush()

	return mgr
}

// Cleanup Performs the necessary cleanup to close the metrics manager.
func (mgr *MetricsManager) Cleanup() {
	mgr.bootFile.Close()
}

// AddBootMetric Writes boot latency breakdown metrics to the boot metric file.
func (mgr *MetricsManager) AddBootMetric(metric *BootMetric) error {
	mgr.bootMutex.Lock()
	defer mgr.bootMutex.Unlock()

	if err := mgr.bootWriter.Write(metric.GetValueLine()); err != nil {
		return err
	}
	mgr.bootWriter.Flush()
	return nil
}