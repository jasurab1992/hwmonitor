package ui

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strings"
	"time"

	"hwmonitor/internal/collectors"
)

const (
	barWidth   = 30
	barFilled  = '█'
	barEmpty   = '░'
	colorReset = "\033[0m"
	colorBold  = "\033[1m"
	colorGreen = "\033[32m"
	colorYellow = "\033[33m"
	colorRed   = "\033[31m"
	colorCyan  = "\033[36m"
	colorWhite = "\033[37m"
	colorDim   = "\033[2m"
	clearScreen = "\033[2J\033[H"
)

// TUI displays hardware metrics in the terminal with periodic refresh.
type TUI struct {
	collectors []collectors.Collector
	interval   time.Duration
}

// NewTUI creates a new TUI instance.
func NewTUI(colls []collectors.Collector, interval time.Duration) *TUI {
	return &TUI{
		collectors: colls,
		interval:   interval,
	}
}

// Run starts the TUI loop. It blocks until the context is cancelled.
func (t *TUI) Run(ctx context.Context) error {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

	// Initial render
	t.render()

	for {
		select {
		case <-ctx.Done():
			fmt.Print(clearScreen)
			fmt.Println("HWmonitor stopped.")
			return nil
		case <-ticker.C:
			t.render()
		}
	}
}

func (t *TUI) render() {
	var sb strings.Builder

	sb.WriteString(clearScreen)

	// Header
	sb.WriteString(colorBold + colorCyan)
	sb.WriteString("╔══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║          HWmonitor — Hardware Monitor                   ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════╝\n")
	sb.WriteString(colorReset)
	sb.WriteString(colorDim + fmt.Sprintf("  Updated: %s    Press Ctrl+C to exit\n", time.Now().Format("15:04:05")) + colorReset)
	sb.WriteString("\n")

	// Collect all metrics
	allMetrics := make(map[string][]collectors.Metric)
	for _, c := range t.collectors {
		metrics, err := c.Collect()
		if err != nil {
			sb.WriteString(fmt.Sprintf("  %s[%s] error: %v%s\n", colorRed, c.Name(), err, colorReset))
			continue
		}
		allMetrics[c.Name()] = metrics
	}

	// CPU section
	if cpuMetrics, ok := allMetrics["cpu"]; ok {
		renderCPUSection(&sb, cpuMetrics)
	}

	// Memory section
	if memMetrics, ok := allMetrics["memory"]; ok {
		renderMemorySection(&sb, memMetrics)
	}

	// Disk section
	if diskMetrics, ok := allMetrics["disk"]; ok {
		renderDiskSection(&sb, diskMetrics)
	}

	// NVMe section
	if nvmeMetrics, ok := allMetrics["nvme"]; ok && len(nvmeMetrics) > 0 {
		renderNVMeSection(&sb, nvmeMetrics)
	}

	fmt.Fprint(os.Stdout, sb.String())
}

func renderCPUSection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("CPU"))

	for _, m := range metrics {
		switch m.Name {
		case "cpu_usage_percent":
			sb.WriteString(fmt.Sprintf("  Total Usage:  %s %5.1f%%\n", progressBar(m.Value, 100), m.Value))
		case "cpu_cores_total":
			sb.WriteString(fmt.Sprintf("  Cores:        %.0f\n", m.Value))
		case "cpu_frequency_mhz":
			sb.WriteString(fmt.Sprintf("  Frequency:    %.0f MHz\n", m.Value))
		}
	}

	// Per-core usage
	type coreUsage struct {
		core string
		pct  float64
	}
	var cores []coreUsage
	for _, m := range metrics {
		if m.Name == "cpu_core_usage_percent" {
			cores = append(cores, coreUsage{core: m.Labels["core"], pct: m.Value})
		}
	}
	sort.Slice(cores, func(i, j int) bool { return cores[i].core < cores[j].core })

	if len(cores) > 0 {
		sb.WriteString("  Per-Core:\n")
		for _, c := range cores {
			sb.WriteString(fmt.Sprintf("    Core %-3s %s %5.1f%%\n", c.core, progressBar(c.pct, 100), c.pct))
		}
	}
	sb.WriteString("\n")
}

func renderMemorySection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("Memory"))

	values := metricMap(metrics)

	if total, ok := values["memory_total_bytes"]; ok {
		used := values["memory_used_bytes"]
		avail := values["memory_available_bytes"]
		pct := values["memory_usage_percent"]
		sb.WriteString(fmt.Sprintf("  RAM:       %s %5.1f%%\n", progressBar(pct, 100), pct))
		sb.WriteString(fmt.Sprintf("             Used: %s / %s  (Available: %s)\n",
			formatBytes(used), formatBytes(total), formatBytes(avail)))
	}

	if swapTotal, ok := values["swap_total_bytes"]; ok && swapTotal > 0 {
		swapUsed := values["swap_used_bytes"]
		swapPct := values["swap_usage_percent"]
		sb.WriteString(fmt.Sprintf("  Swap:      %s %5.1f%%\n", progressBar(swapPct, 100), swapPct))
		sb.WriteString(fmt.Sprintf("             Used: %s / %s\n",
			formatBytes(swapUsed), formatBytes(swapTotal)))
	}
	sb.WriteString("\n")
}

func renderDiskSection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("Disk"))

	// Group usage metrics by mountpoint
	type diskUsage struct {
		device     string
		mountpoint string
		total      float64
		used       float64
		free       float64
		pct        float64
	}
	usageMap := make(map[string]*diskUsage)

	for _, m := range metrics {
		mp := m.Labels["mountpoint"]
		if mp == "" {
			continue
		}
		if _, ok := usageMap[mp]; !ok {
			usageMap[mp] = &diskUsage{
				device:     m.Labels["device"],
				mountpoint: mp,
			}
		}
		du := usageMap[mp]
		switch m.Name {
		case "disk_total_bytes":
			du.total = m.Value
		case "disk_used_bytes":
			du.used = m.Value
		case "disk_free_bytes":
			du.free = m.Value
		case "disk_usage_percent":
			du.pct = m.Value
		}
	}

	// Sort mountpoints
	var mps []string
	for mp := range usageMap {
		mps = append(mps, mp)
	}
	sort.Strings(mps)

	for _, mp := range mps {
		du := usageMap[mp]
		sb.WriteString(fmt.Sprintf("  %-10s %s %5.1f%%  (%s / %s)\n",
			du.mountpoint, progressBar(du.pct, 100), du.pct,
			formatBytes(du.used), formatBytes(du.total)))
	}

	// I/O stats
	type ioStat struct {
		device     string
		readBytes  float64
		writeBytes float64
		readCount  float64
		writeCount float64
	}
	ioMap := make(map[string]*ioStat)

	for _, m := range metrics {
		dev := m.Labels["device"]
		if dev == "" || m.Labels["mountpoint"] != "" {
			continue
		}
		if _, ok := ioMap[dev]; !ok {
			ioMap[dev] = &ioStat{device: dev}
		}
		io := ioMap[dev]
		switch m.Name {
		case "disk_read_bytes_total":
			io.readBytes = m.Value
		case "disk_write_bytes_total":
			io.writeBytes = m.Value
		case "disk_read_count_total":
			io.readCount = m.Value
		case "disk_write_count_total":
			io.writeCount = m.Value
		}
	}

	if len(ioMap) > 0 {
		sb.WriteString("  I/O:\n")
		var devs []string
		for dev := range ioMap {
			devs = append(devs, dev)
		}
		sort.Strings(devs)
		for _, dev := range devs {
			io := ioMap[dev]
			sb.WriteString(fmt.Sprintf("    %-10s R: %s (%s ops)  W: %s (%s ops)\n",
				dev,
				formatBytes(io.readBytes), formatCount(io.readCount),
				formatBytes(io.writeBytes), formatCount(io.writeCount)))
		}
	}
	sb.WriteString("\n")
}

func renderNVMeSection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("NVMe SMART"))

	// Group by device
	type nvmeInfo struct {
		temp     float64
		spare    float64
		used     float64
		hours    float64
		cycles   float64
		unsafe   float64
		mediaErr float64
	}
	devMap := make(map[string]*nvmeInfo)

	for _, m := range metrics {
		dev := m.Labels["device"]
		if _, ok := devMap[dev]; !ok {
			devMap[dev] = &nvmeInfo{}
		}
		info := devMap[dev]
		switch m.Name {
		case "nvme_temperature_celsius":
			info.temp = m.Value
		case "nvme_available_spare_percent":
			info.spare = m.Value
		case "nvme_percentage_used":
			info.used = m.Value
		case "nvme_power_on_hours":
			info.hours = m.Value
		case "nvme_power_cycles":
			info.cycles = m.Value
		case "nvme_unsafe_shutdowns":
			info.unsafe = m.Value
		case "nvme_media_errors":
			info.mediaErr = m.Value
		}
	}

	var devs []string
	for dev := range devMap {
		devs = append(devs, dev)
	}
	sort.Strings(devs)

	for _, dev := range devs {
		info := devMap[dev]
		sb.WriteString(fmt.Sprintf("  %s%s%s\n", colorBold, dev, colorReset))
		sb.WriteString(fmt.Sprintf("    Temperature:    %s%.0f°C%s\n", tempColor(info.temp), info.temp, colorReset))
		sb.WriteString(fmt.Sprintf("    Available Spare: %.0f%%\n", info.spare))
		sb.WriteString(fmt.Sprintf("    Wear (Used):    %.0f%%\n", info.used))
		sb.WriteString(fmt.Sprintf("    Power On Hours: %.0f h\n", info.hours))
		sb.WriteString(fmt.Sprintf("    Power Cycles:   %.0f\n", info.cycles))
		sb.WriteString(fmt.Sprintf("    Unsafe Shutdowns: %.0f\n", info.unsafe))
		sb.WriteString(fmt.Sprintf("    Media Errors:   %.0f\n", info.mediaErr))
	}
	sb.WriteString("\n")
}

// Helper functions

func sectionHeader(name string) string {
	return fmt.Sprintf("%s%s── %s ──%s\n", colorBold, colorCyan, name, colorReset)
}

func progressBar(value, max float64) string {
	ratio := value / max
	if ratio > 1 {
		ratio = 1
	}
	if ratio < 0 {
		ratio = 0
	}
	filled := int(math.Round(ratio * barWidth))
	empty := barWidth - filled

	color := colorGreen
	if ratio > 0.8 {
		color = colorRed
	} else if ratio > 0.6 {
		color = colorYellow
	}

	return fmt.Sprintf("%s%s%s%s%s",
		color,
		strings.Repeat(string(barFilled), filled),
		colorDim,
		strings.Repeat(string(barEmpty), empty),
		colorReset)
}

func formatBytes(b float64) string {
	units := []string{"B", "KB", "MB", "GB", "TB"}
	i := 0
	for b >= 1024 && i < len(units)-1 {
		b /= 1024
		i++
	}
	return fmt.Sprintf("%.1f %s", b, units[i])
}

func formatCount(c float64) string {
	if c >= 1e9 {
		return fmt.Sprintf("%.1fG", c/1e9)
	}
	if c >= 1e6 {
		return fmt.Sprintf("%.1fM", c/1e6)
	}
	if c >= 1e3 {
		return fmt.Sprintf("%.1fK", c/1e3)
	}
	return fmt.Sprintf("%.0f", c)
}

func metricMap(metrics []collectors.Metric) map[string]float64 {
	m := make(map[string]float64)
	for _, metric := range metrics {
		m[metric.Name] = metric.Value
	}
	return m
}

func tempColor(temp float64) string {
	if temp >= 70 {
		return colorRed
	}
	if temp >= 50 {
		return colorYellow
	}
	return colorGreen
}
