/*
 * This file is part of the KubeVirt project
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *     http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 * Copyright The KubeVirt Authors.
 */

package domainstats

import (
	"fmt"

	"github.com/rhobs/operator-observability-toolkit/pkg/operatormetrics"
)

var (
	gpuAgentStatus = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_agent_status",
			Help: "Indicates the guest GPU metrics agent status.",
		},
	)
	gpuUtilization = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_utilization_percent",
			Help: "GPU compute utilization percentage (0-100) as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuMemoryUtilization = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_memory_utilization_percent",
			Help: "GPU memory controller utilization percentage (0-100) as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuMemoryUsedBytes = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_memory_used_bytes",
			Help: "GPU memory used in bytes as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuMemoryTotalBytes = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_memory_total_bytes",
			Help: "GPU total memory in bytes as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuTemperatureCelsius = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_temperature_celsius",
			Help: "GPU temperature in degrees Celsius as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuPowerUsageMilliwatts = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_power_usage_milliwatts",
			Help: "GPU power usage in milliwatts as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuECCErrorsSingleBit = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_ecc_errors_single_bit_total",
			Help: "GPU lifetime corrected (single-bit) ECC error count as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuECCErrorsDoubleBit = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_ecc_errors_double_bit_total",
			Help: "GPU lifetime uncorrected (double-bit) ECC error count as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuEncoderUtilization = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_encoder_utilization_percent",
			Help: "GPU video encoder utilization percentage (0-100) as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuDecoderUtilization = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_decoder_utilization_percent",
			Help: "GPU video decoder utilization percentage (0-100) as reported by the guest GPU metrics agent via NVML.",
		},
	)
	gpuRunningProcesses = operatormetrics.NewGauge(
		operatormetrics.MetricOpts{
			Name: "kubevirt_vmi_gpu_running_processes",
			Help: "Number of compute processes running on the GPU as reported by the guest GPU metrics agent via NVML.",
		},
	)
)

// GPUMetricsResponse is the top-level JSON structure from the guest agent.
type GPUMetricsResponse struct {
	Version string        `json:"version"`
	Error   *GPUError     `json:"error,omitempty"`
	Devices []GPUDevStats `json:"devices,omitempty"`
}

// GPUError describes an NVML error reported by the guest agent.
type GPUError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// GPUDevStats mirrors the per-GPU JSON structure emitted by the guest GPU metrics agent.
type GPUDevStats struct {
	Index                     uint32 `json:"index"`
	UUID                      string `json:"uuid"`
	Name                      string `json:"name"`
	GPUUtilizationPercent     uint32 `json:"gpuUtilizationPercent"`
	MemoryUtilizationPercent  uint32 `json:"memoryUtilizationPercent"`
	MemoryUsedBytes           uint64 `json:"memoryUsedBytes"`
	MemoryTotalBytes          uint64 `json:"memoryTotalBytes"`
	TemperatureCelsius        uint32 `json:"temperatureCelsius"`
	PowerUsageMilliwatts      uint32 `json:"powerUsageMilliwatts"`
	PowerLimitMilliwatts      uint32 `json:"powerLimitMilliwatts"`
	ECCErrorsSingleBit        uint64 `json:"eccErrorsSingleBit"`
	ECCErrorsDoubleBit        uint64 `json:"eccErrorsDoubleBit"`
	EncoderUtilizationPercent uint32 `json:"encoderUtilizationPercent"`
	DecoderUtilizationPercent uint32 `json:"decoderUtilizationPercent"`
	RunningProcesses          uint32 `json:"runningProcesses"`
	PCIeTxBytesPerSecond      uint64 `json:"pcieTxBytesPerSecond"`
	PCIeRxBytesPerSecond      uint64 `json:"pcieRxBytesPerSecond"`
}

type gpuMetrics struct{}

func (gpuMetrics) Describe() []operatormetrics.Metric {
	return []operatormetrics.Metric{
		gpuAgentStatus,
		gpuUtilization,
		gpuMemoryUtilization,
		gpuMemoryUsedBytes,
		gpuMemoryTotalBytes,
		gpuTemperatureCelsius,
		gpuPowerUsageMilliwatts,
		gpuECCErrorsSingleBit,
		gpuECCErrorsDoubleBit,
		gpuEncoderUtilization,
		gpuDecoderUtilization,
		gpuRunningProcesses,
	}
}

func (gpuMetrics) Collect(vmiReport *VirtualMachineInstanceReport) []operatormetrics.CollectorResult {
	if vmiReport.vmiStats.GPUStats == nil {
		return nil
	}

	var crs []operatormetrics.CollectorResult

	scrape := vmiReport.vmiStats.GPUStats

	statusLabels := map[string]string{"version": scrape.Version}
	statusValue := 0.0
	if scrape.Error != nil {
		statusValue = float64(scrape.Error.Code)
		statusLabels["error_code"] = fmt.Sprintf("%d", scrape.Error.Code)
		statusLabels["error_message"] = scrape.Error.Message
	}
	crs = append(crs, vmiReport.newCollectorResultWithLabels(gpuAgentStatus, statusValue, statusLabels))

	if scrape.Error != nil {
		return crs
	}

	for _, gpu := range scrape.Devices {
		gpuLabels := map[string]string{
			"gpu_index": fmt.Sprintf("%d", gpu.Index),
			"gpu_uuid":  gpu.UUID,
			"gpu_name":  gpu.Name,
		}

		crs = append(crs,
			vmiReport.newCollectorResultWithLabels(gpuUtilization, float64(gpu.GPUUtilizationPercent), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuMemoryUtilization, float64(gpu.MemoryUtilizationPercent), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuMemoryUsedBytes, float64(gpu.MemoryUsedBytes), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuMemoryTotalBytes, float64(gpu.MemoryTotalBytes), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuTemperatureCelsius, float64(gpu.TemperatureCelsius), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuPowerUsageMilliwatts, float64(gpu.PowerUsageMilliwatts), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuECCErrorsSingleBit, float64(gpu.ECCErrorsSingleBit), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuECCErrorsDoubleBit, float64(gpu.ECCErrorsDoubleBit), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuEncoderUtilization, float64(gpu.EncoderUtilizationPercent), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuDecoderUtilization, float64(gpu.DecoderUtilizationPercent), gpuLabels),
			vmiReport.newCollectorResultWithLabels(gpuRunningProcesses, float64(gpu.RunningProcesses), gpuLabels),
		)
	}

	return crs
}
