package ui

import (
	"context"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"hwmonitor/internal/collectors"
)

const (
	barWidth    = 28
	barFilled   = '█'
	barEmpty    = '░'
	colorReset  = "\033[0m"
	colorBold   = "\033[1m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorRed    = "\033[31m"
	colorCyan   = "\033[36m"
	colorWhite  = "\033[37m"
	colorDim    = "\033[2m"
	clearScreen = "\033[2J\033[H"
)

// TUI displays hardware metrics in the terminal with periodic refresh.
type TUI struct {
	collectors []collectors.Collector
	interval   time.Duration
}

func NewTUI(colls []collectors.Collector, interval time.Duration) *TUI {
	return &TUI{collectors: colls, interval: interval}
}

func (t *TUI) Run(ctx context.Context) error {
	ticker := time.NewTicker(t.interval)
	defer ticker.Stop()

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
	sb.WriteString(colorBold + colorCyan)
	sb.WriteString("╔══════════════════════════════════════════════════════════╗\n")
	sb.WriteString("║           HWmonitor — Hardware Monitor                  ║\n")
	sb.WriteString("╚══════════════════════════════════════════════════════════╝\n")
	sb.WriteString(colorReset)
	sb.WriteString(colorDim + fmt.Sprintf("  Updated: %s    Press Ctrl+C to exit\n", time.Now().Format("15:04:05")) + colorReset)
	sb.WriteString("\n")

	allMetrics := make(map[string][]collectors.Metric)
	for _, c := range t.collectors {
		metrics, err := c.Collect()
		if err != nil {
			sb.WriteString(fmt.Sprintf("  %s[%s] error: %v%s\n", colorRed, c.Name(), err, colorReset))
			continue
		}
		allMetrics[c.Name()] = metrics
	}

	if m, ok := allMetrics["sysinfo"]; ok && len(m) > 0 {
		renderSysInfoSection(&sb, m)
	}
	if m, ok := allMetrics["cpu"]; ok {
		renderCPUSection(&sb, m)
	}
	if m, ok := allMetrics["memory"]; ok {
		renderMemorySection(&sb, m)
	}
	// Temperatures: prefer sensors (LHM/OHM per-core), fall back to cpu_temp (ACPI zones)
	sensorsM := allMetrics["sensors"]
	cpuTempM := allMetrics["cpu_temp"]
	nvmeM := allMetrics["nvme"]
	smartM := allMetrics["smart"]
	ipmiM := allMetrics["ipmi"]
	renderTemperaturesSection(&sb, sensorsM, cpuTempM, nvmeM, smartM, ipmiM)

	if len(sensorsM) > 0 {
		renderVoltagesSection(&sb, sensorsM)
		renderFansSection(&sb, sensorsM)
	}
	if m, ok := allMetrics["disk"]; ok {
		renderDiskSection(&sb, m)
	}
	if len(nvmeM) > 0 {
		renderNVMeSmartSection(&sb, nvmeM)
	}
	if len(smartM) > 0 {
		renderSATASmartSection(&sb, smartM)
	}
	if m, ok := allMetrics["network"]; ok && len(m) > 0 {
		renderNetworkSection(&sb, m)
	}

	fmt.Fprint(os.Stdout, sb.String())
}

// ─── Section renderers ─────────────────────────────────────────────────────

func renderSysInfoSection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("System Info"))

	for _, m := range metrics {
		switch m.Name {
		case "sysinfo_baseboard_info":
			sb.WriteString(fmt.Sprintf("  Motherboard: %s %s\n",
				m.Labels["manufacturer"], m.Labels["product"]))
		case "sysinfo_bios_info":
			sb.WriteString(fmt.Sprintf("  BIOS:        %s\n", m.Labels["version"]))
		case "sysinfo_cpu_cores":
			sb.WriteString(fmt.Sprintf("  CPU:         %s\n", m.Labels["processor"]))
			sb.WriteString(fmt.Sprintf("               %d cores", int(m.Value)))
		case "sysinfo_cpu_threads":
			sb.WriteString(fmt.Sprintf(" / %d threads\n", int(m.Value)))
		}
	}

	// RAM modules
	var ramLines []string
	totalRAM := 0.0
	for _, m := range metrics {
		if m.Name == "sysinfo_memory_module_bytes" {
			gb := m.Value / (1024 * 1024 * 1024)
			totalRAM += gb
			ramLines = append(ramLines, fmt.Sprintf("    Slot %s: %.0f GB %s @ %s MHz",
				m.Labels["slot"], gb, m.Labels["type"], m.Labels["speed_mhz"]))
		}
	}
	if len(ramLines) > 0 {
		sb.WriteString(fmt.Sprintf("  RAM:         %.0f GB total\n", totalRAM))
		for _, l := range ramLines {
			sb.WriteString(l + "\n")
		}
	}
	sb.WriteString("\n")
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

	type coreUsage struct{ core string; pct float64 }
	var cores []coreUsage
	for _, m := range metrics {
		if m.Name == "cpu_core_usage_percent" {
			cores = append(cores, coreUsage{m.Labels["core"], m.Value})
		}
	}
	sort.Slice(cores, func(i, j int) bool {
		ni, ei := strconv.Atoi(cores[i].core)
		nj, ej := strconv.Atoi(cores[j].core)
		if ei == nil && ej == nil {
			return ni < nj
		}
		return cores[i].core < cores[j].core
	})
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
		sb.WriteString(fmt.Sprintf("  RAM:   %s %5.1f%%\n", progressBar(pct, 100), pct))
		sb.WriteString(fmt.Sprintf("         Used: %s / %s  (Free: %s)\n",
			formatBytes(used), formatBytes(total), formatBytes(avail)))
	}
	if swapTotal, ok := values["swap_total_bytes"]; ok && swapTotal > 0 {
		swapUsed := values["swap_used_bytes"]
		swapPct := values["swap_usage_percent"]
		sb.WriteString(fmt.Sprintf("  Swap:  %s %5.1f%%\n", progressBar(swapPct, 100), swapPct))
		sb.WriteString(fmt.Sprintf("         Used: %s / %s\n", formatBytes(swapUsed), formatBytes(swapTotal)))
	}
	sb.WriteString("\n")
}

func renderTemperaturesSection(sb *strings.Builder, sensorsM, cpuTempM, nvmeM, smartM, ipmiM []collectors.Metric) {
	var lines []string

	// From sensors (LHM/OHM) — per-core CPU temps
	for _, m := range sensorsM {
		if m.Name == "sensor_temperature_celsius" {
			lines = append(lines, fmt.Sprintf("  %-30s %s%.0f°C%s",
				m.Labels["name"], tempColor(m.Value), m.Value, colorReset))
		}
	}

	// Fallback: ACPI thermal zones (if no LHM sensor temps)
	if len(lines) == 0 {
		for _, m := range cpuTempM {
			if m.Name == "cpu_temp_celsius" {
				zone := m.Labels["zone"]
				src := m.Labels["source"]
				lines = append(lines, fmt.Sprintf("  %-30s %s%.1f°C%s  (%s)",
					zone, tempColor(m.Value), m.Value, colorReset, src))
			}
		}
	}

	// NVMe temps
	seen := map[string]bool{}
	for _, m := range nvmeM {
		if m.Name == "nvme_temperature_celsius" {
			dev := m.Labels["device"]
			if !seen[dev] {
				seen[dev] = true
				lines = append(lines, fmt.Sprintf("  %-30s %s%.0f°C%s",
					"NVMe "+dev, tempColor(m.Value), m.Value, colorReset))
			}
		}
	}

	// SATA/HDD temps
	for _, m := range smartM {
		if m.Name == "smart_temp_celsius" {
			dev := m.Labels["device"]
			lines = append(lines, fmt.Sprintf("  %-30s %s%.0f°C%s",
				"SATA "+dev, tempColor(m.Value), m.Value, colorReset))
		}
	}

	// IPMI BMC sensors (inlet, ambient, exhaust, etc.)
	for _, m := range ipmiM {
		if m.Name == "ipmi_temperature_celsius" {
			lines = append(lines, fmt.Sprintf("  %-30s %s%.0f°C%s",
				m.Labels["sensor"], tempColor(m.Value), m.Value, colorReset))
		}
	}

	if len(lines) == 0 {
		return
	}
	sb.WriteString(sectionHeader("Temperatures"))
	for _, l := range lines {
		sb.WriteString(l + "\n")
	}
	sb.WriteString("\n")
}

func renderVoltagesSection(sb *strings.Builder, metrics []collectors.Metric) {
	var lines []string
	for _, m := range metrics {
		if m.Name == "sensor_voltage_volts" {
			lines = append(lines, fmt.Sprintf("  %-30s %.3f V", m.Labels["name"], m.Value))
		}
	}
	if len(lines) == 0 {
		return
	}
	sb.WriteString(sectionHeader("Voltages"))
	for _, l := range lines {
		sb.WriteString(l + "\n")
	}
	sb.WriteString("\n")
}

func renderFansSection(sb *strings.Builder, metrics []collectors.Metric) {
	var lines []string
	for _, m := range metrics {
		if m.Name == "sensor_fan_rpm" && m.Value > 0 {
			lines = append(lines, fmt.Sprintf("  %-30s %.0f RPM", m.Labels["name"], m.Value))
		}
	}
	if len(lines) == 0 {
		return
	}
	sb.WriteString(sectionHeader("Fans"))
	for _, l := range lines {
		sb.WriteString(l + "\n")
	}
	sb.WriteString("\n")
}

func renderDiskSection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("Disk"))

	type diskUsage struct {
		device, mountpoint string
		total, used, free, pct float64
	}
	usageMap := make(map[string]*diskUsage)
	for _, m := range metrics {
		mp := m.Labels["mountpoint"]
		if mp == "" {
			continue
		}
		if _, ok := usageMap[mp]; !ok {
			usageMap[mp] = &diskUsage{device: m.Labels["device"], mountpoint: mp}
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

	type ioStat struct {
		device             string
		readBytes, writeBytes float64
		readCount, writeCount float64
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
		sb.WriteString("  I/O totals:\n")
		var devs []string
		for dev := range ioMap {
			devs = append(devs, dev)
		}
		sort.Strings(devs)
		for _, dev := range devs {
			io := ioMap[dev]
			sb.WriteString(fmt.Sprintf("    %-12s R: %-10s  W: %s\n",
				dev, formatBytes(io.readBytes), formatBytes(io.writeBytes)))
		}
	}
	sb.WriteString("\n")
}

func renderNVMeSmartSection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("NVMe SMART"))

	type nvmeInfo struct {
		used, spare, hours, mediaErrors float64
		hasSpare                        bool
	}
	devMap := make(map[string]*nvmeInfo)
	for _, m := range metrics {
		dev := m.Labels["device"]
		if _, ok := devMap[dev]; !ok {
			devMap[dev] = &nvmeInfo{}
		}
		info := devMap[dev]
		switch m.Name {
		case "nvme_percentage_used":
			info.used = m.Value
		case "nvme_available_spare_percent":
			info.spare = m.Value
			info.hasSpare = true
		case "nvme_power_on_hours":
			info.hours = m.Value
		case "nvme_media_errors_total":
			info.mediaErrors = m.Value
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
		sb.WriteString(fmt.Sprintf("    Wear used:      %.0f%%\n", info.used))
		if info.hasSpare {
			spareColor := colorGreen
			if info.spare < 10 {
				spareColor = colorRed
			} else if info.spare < 30 {
				spareColor = colorYellow
			}
			sb.WriteString(fmt.Sprintf("    Available spare:%s%.0f%%%s\n", spareColor, info.spare, colorReset))
		}
		sb.WriteString(fmt.Sprintf("    Power On Hours: %.0f h\n", info.hours))
		errColor := colorGreen
		if info.mediaErrors > 0 {
			errColor = colorYellow
		}
		sb.WriteString(fmt.Sprintf("    Media errors:   %s%.0f%s\n", errColor, info.mediaErrors, colorReset))
	}
	sb.WriteString("\n")
}

func renderSATASmartSection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("SATA SMART"))

	type smartInfo struct {
		lifeRemaining    float64
		hasLife          bool
		hours            float64
		hasHours         bool
		reallocated      float64
		hasReallocated   bool
		pending          float64
		hasPending       bool
	}
	devMap := make(map[string]*smartInfo)
	for _, m := range metrics {
		dev := m.Labels["device"]
		if _, ok := devMap[dev]; !ok {
			devMap[dev] = &smartInfo{}
		}
		info := devMap[dev]
		switch m.Name {
		case "smart_life_remaining_percent":
			info.lifeRemaining = m.Value
			info.hasLife = true
		case "smart_power_on_hours":
			info.hours = m.Value
			info.hasHours = true
		case "smart_reallocated_sectors":
			info.reallocated = m.Value
			info.hasReallocated = true
		case "smart_pending_sectors":
			info.pending = m.Value
			info.hasPending = true
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
		if info.hasLife {
			sb.WriteString(fmt.Sprintf("    Life remaining: %.0f%%\n", info.lifeRemaining))
		}
		if info.hasHours {
			sb.WriteString(fmt.Sprintf("    Power On Hours: %.0f h\n", info.hours))
		}
		if info.hasReallocated {
			color := colorGreen
			if info.reallocated > 0 {
				color = colorYellow
			}
			sb.WriteString(fmt.Sprintf("    Reallocated:    %s%.0f%s sectors\n", color, info.reallocated, colorReset))
		}
		if info.hasPending {
			color := colorGreen
			if info.pending > 0 {
				color = colorYellow
			}
			sb.WriteString(fmt.Sprintf("    Pending:        %s%.0f%s sectors\n", color, info.pending, colorReset))
		}
	}
	sb.WriteString("\n")
}

func renderNetworkSection(sb *strings.Builder, metrics []collectors.Metric) {
	sb.WriteString(sectionHeader("Network"))

	type ifStat struct {
		sent, recv float64
	}
	ifMap := make(map[string]*ifStat)
	for _, m := range metrics {
		iface := m.Labels["interface"]
		if _, ok := ifMap[iface]; !ok {
			ifMap[iface] = &ifStat{}
		}
		switch m.Name {
		case "network_bytes_sent_total":
			ifMap[iface].sent = m.Value
		case "network_bytes_recv_total":
			ifMap[iface].recv = m.Value
		}
	}

	var ifaces []string
	for iface := range ifMap {
		ifaces = append(ifaces, iface)
	}
	sort.Strings(ifaces)

	for _, iface := range ifaces {
		st := ifMap[iface]
		if st.sent == 0 && st.recv == 0 {
			continue
		}
		sb.WriteString(fmt.Sprintf("  %-20s  ↑ %-10s  ↓ %s\n",
			iface, formatBytes(st.sent), formatBytes(st.recv)))
	}
	sb.WriteString("\n")
}

// ─── Helpers ───────────────────────────────────────────────────────────────

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
