package watchdog

import (
	"context"
	"log/slog"
	"time"

	"github.com/TwiN/gatus/v5/config"
	"github.com/TwiN/gatus/v5/config/endpoint"
	"github.com/TwiN/gatus/v5/config/state"
	"github.com/TwiN/gatus/v5/logging"
	"github.com/TwiN/gatus/v5/metrics"
	"github.com/TwiN/gatus/v5/storage/store"
)

type maintenanceStatus int

const (
	noMaintenance maintenanceStatus = iota
	endpointMaintenance
	globalMaintenance
)

// monitorEndpoint a single endpoint in a loop
func monitorEndpoint(ep *endpoint.Endpoint, cfg *config.Config, extraLabels []string, ctx context.Context) {
	// Run it immediately on start
	executeEndpoint(ep, cfg, extraLabels)
	// Loop for the next executions
	ticker := time.NewTicker(ep.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			slog.Warn("Canceling current execution", ep.GetLogAttribute())
			return
		case <-ticker.C:
			executeEndpoint(ep, cfg, extraLabels)
		}
	}
	// Just in case somebody wandered all the way to here and wonders, "what about ExternalEndpoints?"
	// Alerting is checked every time an external endpoint is pushed to Gatus, so they're not monitored
	// periodically like they are for normal endpoints.
}

func executeEndpoint(ep *endpoint.Endpoint, cfg *config.Config, extraLabels []string) {
	logger := slog.With(ep.GetLogAttribute())

	// Acquire semaphore to limit concurrent endpoint monitoring
	if err := monitoringSemaphore.Acquire(ctx, 1); err != nil {
		// Only fails if context is cancelled (during shutdown)
		logger.Debug("Context cancelled; skipping execution", "error", err.Error())
		return
	}
	defer monitoringSemaphore.Release(1)
	// If there's a connectivity checker configured, check if Gatus has internet connectivity
	if cfg.Connectivity != nil && cfg.Connectivity.Checker != nil && !cfg.Connectivity.Checker.IsConnected() {
		logger.Info("No connectivity, skipping execution")
		return
	}

	logger.Debug("Monitoring start")
	result := ep.EvaluateHealth()
	maintenanceState := GetMaintenanceStatus(ep, cfg)
	if maintenanceState != noMaintenance && !result.Success {
		result.State = state.DefaultMaintenanceStateName
	}
	// TODO#227 Evaluate result.Success based on set states' healthiness configuration once that config option is implemented
	if cfg.Metrics {
		metrics.PublishMetricsForEndpoint(ep, result, extraLabels)
	}
	UpdateEndpointStatus(ep, result)
	if logging.Level() <= slog.LevelDebug && !result.Success {
		logger.Debug("Monitoring done with errors", result.GetLogAttribute(), "body", result.Body)
	} else {
		logger.Info("Monitoring done", result.GetLogAttribute())
	}
	if maintenanceState == noMaintenance {
		HandleAlerting(ep, result, cfg.Alerting)
	} else {
		logger.Debug("Not handling alerting due to maintenance window")
	}
	logger.Debug("Wait for next monitoring", "interval", ep.Interval)
}

func GetMaintenanceStatus(ep *endpoint.Endpoint, cfg *config.Config) maintenanceStatus {
	if cfg.Maintenance.IsUnderMaintenance() {
		return globalMaintenance
	}
	for _, maintenanceWindow := range ep.MaintenanceWindows {
		if maintenanceWindow.IsUnderMaintenance() {
			return endpointMaintenance
		}
	}
	return noMaintenance
}

func GetMaintenanceStatusName(status maintenanceStatus) string {
	switch status {
	case globalMaintenance:
		return "global"
	case endpointMaintenance:
		return "endpoint"
	default:
		return "no"
	}
}

// UpdateEndpointStatus persists the endpoint result in the storage
func UpdateEndpointStatus(ep *endpoint.Endpoint, result *endpoint.Result) {
	if err := store.Get().InsertEndpointResult(ep, result); err != nil {
		slog.Error("Failed to insert result in storage", "error", err.Error())
	}
}
