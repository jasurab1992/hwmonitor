package main

import (
	"context"
	"fmt"
	"strings"
	"time"

	"hwmonitor/internal/collectors"
	"hwmonitor/internal/config"
)

// ── Data types exposed to the frontend ──────────────────────────────────────

type CoreUsage struct {
	Core  string  `json:"core"`
	Usage float64 `json:"usage"`
}

type CPUData struct {
	TotalUsage float64     `json:"totalUsage"`
	Cores      float64     `json:"cores"`
	FreqMHz    float64     `json:"freqMHz"`
	PerCore    []CoreUsage `json:"perCore"`
}

type MemoryData struct {
	TotalBytes     float64 `json:"totalBytes"`
	UsedBytes      float64 `json:"usedBytes"`
	AvailBytes     float64 `json:"availBytes"`
	UsagePercent   float64 `json:"usagePercent"`
	SwapTotalBytes float64 `json:"swapTotalBytes"`
	SwapUsedBytes  float64 `json:"swapUsedBytes"`
	SwapPercent    float64 `json:"swapPercent"`
}

type TempEntry struct {
	Name  string  `json:"name"`
	TempC float64 `json:"tempC"`
}

type VoltageEntry struct {
	Name   string  `json:"name"`
	Volts  float64 `json:"volts"`
}

type FanEntry struct {
	Name string  `json:"name"`
	RPM  float64 `json:"rpm"`
}

type DiskUsage struct {
	Mountpoint   string  `json:"mountpoint"`
	Device       string  `json:"device"`
	TotalBytes   float64 `json:"totalBytes"`
	UsedBytes    float64 `json:"usedBytes"`
	FreeBytes    float64 `json:"freeBytes"`
	UsagePercent float64 `json:"usagePercent"`
}

type DiskIO struct {
	Device      string  `json:"device"`
	ReadBytes   float64 `json:"readBytes"`
	WriteBytes  float64 `json:"writeBytes"`
}

type NVMeEntry struct {
	Device        string  `json:"device"`
	LifeRemaining float64 `json:"lifeRemaining"`
	SpareAvail    float64 `json:"spareAvail"`
	HasSpare      bool    `json:"hasSpare"`
	PowerOnHours  float64 `json:"powerOnHours"`
	MediaErrors   float64 `json:"mediaErrors"`
	TempC         float64 `json:"tempC"`
	HasTemp       bool    `json:"hasTemp"`
}

type SATAEntry struct {
	Device         string  `json:"device"`
	LifeRemaining  float64 `json:"lifeRemaining"`
	HasLife        bool    `json:"hasLife"`
	SpareAvail     float64 `json:"spareAvail"`
	HasSpare       bool    `json:"hasSpare"`
	PowerOnHours   float64 `json:"powerOnHours"`
	HasHours       bool    `json:"hasHours"`
	Reallocated    float64 `json:"reallocated"`
	HasReallocated bool    `json:"hasReallocated"`
	Pending        float64 `json:"pending"`
	HasPending     bool    `json:"hasPending"`
	TempC          float64 `json:"tempC"`
	HasTemp        bool    `json:"hasTemp"`
}

type NetworkEntry struct {
	Interface string  `json:"interface"`
	SentBytes float64 `json:"sentBytes"`
	RecvBytes float64 `json:"recvBytes"`
}

type RAMSlot struct {
	Slot     string  `json:"slot"`
	Bytes    float64 `json:"bytes"`
	Type     string  `json:"type"`
	SpeedMHz string  `json:"speedMHz"`
}

type SysInfoData struct {
	CPU         string    `json:"cpu"`
	Cores       int       `json:"cores"`
	Threads     int       `json:"threads"`
	Motherboard string    `json:"motherboard"`
	BIOS        string    `json:"bios"`
	RAMTotal    float64   `json:"ramTotal"`
	RAMSlots    []RAMSlot `json:"ramSlots"`
}

type Snapshot struct {
	Timestamp int64          `json:"timestamp"`
	SysInfo   SysInfoData    `json:"sysinfo"`
	CPU       CPUData        `json:"cpu"`
	Memory    MemoryData     `json:"memory"`
	Temps     []TempEntry    `json:"temps"`
	Voltages  []VoltageEntry `json:"voltages"`
	Fans      []FanEntry     `json:"fans"`
	DiskUsage []DiskUsage    `json:"diskUsage"`
	DiskIO    []DiskIO       `json:"diskIO"`
	NVMe      []NVMeEntry    `json:"nvme"`
	SATA      []SATAEntry    `json:"sata"`
	Network   []NetworkEntry `json:"network"`
}

// ── App ─────────────────────────────────────────────────────────────────────

type App struct {
	ctx        context.Context
	collectors []collectors.Collector
}

func NewApp() *App {
	cfg, err := config.LoadConfig("config.yaml")
	if err != nil {
		// fallback defaults
		cfg = &config.Config{}
		cfg.Collectors.CPU = true
		cfg.Collectors.Memory = true
		cfg.Collectors.Disk = true
		cfg.Collectors.NVMe = true
		cfg.Collectors.CPUTemp = true
		cfg.Collectors.SMART = true
		cfg.Collectors.Network = true
		cfg.Collectors.SysInfo = true
		cfg.Collectors.Sensors = true
		cfg.Collectors.IPMI = true
		cfg.Collectors.LHM = true
	}

	var colls []collectors.Collector
	if cfg.Collectors.SysInfo {
		colls = append(colls, collectors.NewSysInfoCollector())
	}
	if cfg.Collectors.CPU {
		colls = append(colls, collectors.NewCPUCollector())
	}
	if cfg.Collectors.Memory {
		colls = append(colls, collectors.NewMemoryCollector())
	}
	if cfg.Collectors.Disk {
		colls = append(colls, collectors.NewDiskCollector())
	}
	if cfg.Collectors.NVMe {
		colls = append(colls, collectors.NewNVMeCollector())
	}
	if cfg.Collectors.CPUTemp {
		colls = append(colls, collectors.NewCPUTempCollector())
	}
	if cfg.Collectors.SMART {
		colls = append(colls, collectors.NewSMARTCollector())
	}
	if cfg.Collectors.Network {
		colls = append(colls, collectors.NewNetworkCollector())
	}
	if cfg.Collectors.Sensors {
		colls = append(colls, collectors.NewSensorsCollector())
	}
	if cfg.Collectors.IPMI {
		colls = append(colls, collectors.NewIPMICollector())
	}
	if cfg.Collectors.LHM {
		colls = append(colls, collectors.NewLHMCollector())
	}

	return &App{collectors: colls}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) shutdown(ctx context.Context) {
	collectors.CleanupRing0()
	collectors.CleanupSmartctl()
	collectors.CleanupIPMI()
	collectors.CleanupLHM()
}

// GetSnapshot collects all metrics and returns a structured Snapshot.
func (a *App) GetSnapshot() Snapshot {
	raw := make(map[string][]collectors.Metric)
	for _, c := range a.collectors {
		m, err := c.Collect()
		if err == nil {
			raw[c.Name()] = m
		}
	}

	snap := Snapshot{Timestamp: time.Now().UnixMilli()}
	snap.SysInfo = buildSysInfo(raw["sysinfo"])
	snap.CPU = buildCPU(raw["cpu"])
	snap.Memory = buildMemory(raw["memory"])
	snap.Temps = buildTemps(raw["sensors"], raw["cpu_temp"], raw["nvme"], raw["smart"], raw["ipmi"], raw["lhm"])
	snap.Voltages = buildVoltages(raw["sensors"], raw["lhm"], raw["ipmi"])
	snap.Fans = buildFans(raw["sensors"], raw["lhm"], raw["ipmi"])
	snap.DiskUsage, snap.DiskIO = buildDisk(raw["disk"])
	snap.NVMe = buildNVMe(raw["nvme"])
	snap.SATA = buildSATA(raw["smart"])
	snap.Network = buildNetwork(raw["network"])
	return snap
}

// ── Builders ────────────────────────────────────────────────────────────────

func buildSysInfo(m []collectors.Metric) SysInfoData {
	var s SysInfoData
	for _, metric := range m {
		switch metric.Name {
		case "sysinfo_cpu_cores":
			s.CPU = metric.Labels["processor"]
			s.Cores = int(metric.Value)
		case "sysinfo_cpu_threads":
			s.Threads = int(metric.Value)
		case "sysinfo_baseboard_info":
			s.Motherboard = metric.Labels["manufacturer"] + " " + metric.Labels["product"]
		case "sysinfo_bios_info":
			s.BIOS = metric.Labels["version"]
		case "sysinfo_memory_module_bytes":
			s.RAMTotal += metric.Value
			s.RAMSlots = append(s.RAMSlots, RAMSlot{
				Slot:     metric.Labels["slot"],
				Bytes:    metric.Value,
				Type:     metric.Labels["type"],
				SpeedMHz: metric.Labels["speed_mhz"],
			})
		}
	}
	return s
}

func buildCPU(m []collectors.Metric) CPUData {
	var d CPUData
	coreMap := map[string]float64{}
	for _, metric := range m {
		switch metric.Name {
		case "cpu_usage_percent":
			d.TotalUsage = metric.Value
		case "cpu_cores_total":
			d.Cores = metric.Value
		case "cpu_frequency_mhz":
			d.FreqMHz = metric.Value
		case "cpu_core_usage_percent":
			coreMap[metric.Labels["core"]] = metric.Value
		}
	}
	for core, pct := range coreMap {
		d.PerCore = append(d.PerCore, CoreUsage{Core: core, Usage: pct})
	}
	return d
}

func buildMemory(m []collectors.Metric) MemoryData {
	var d MemoryData
	for _, metric := range m {
		switch metric.Name {
		case "memory_total_bytes":
			d.TotalBytes = metric.Value
		case "memory_used_bytes":
			d.UsedBytes = metric.Value
		case "memory_available_bytes":
			d.AvailBytes = metric.Value
		case "memory_usage_percent":
			d.UsagePercent = metric.Value
		case "swap_total_bytes":
			d.SwapTotalBytes = metric.Value
		case "swap_used_bytes":
			d.SwapUsedBytes = metric.Value
		case "swap_usage_percent":
			d.SwapPercent = metric.Value
		}
	}
	return d
}

func buildTemps(sensorsM, cpuTempM, nvmeM, smartM, ipmiM, lhmM []collectors.Metric) []TempEntry {
	var out []TempEntry

	for _, m := range sensorsM {
		if m.Name == "sensor_temperature_celsius" {
			out = append(out, TempEntry{m.Labels["name"], m.Value})
		}
	}

	if len(out) == 0 {
		for _, m := range lhmM {
			if m.Name != "lhm_temperature_celsius" {
				continue
			}
			name := m.Labels["name"]
			if strings.Contains(name, "Distance to TjMax") ||
				strings.HasPrefix(name, "CPU Core #") ||
				strings.HasPrefix(name, "GPU Core #") {
				continue
			}
			hw := m.Labels["hardware"]
			id := m.Labels["identifier"]
			prefix := ""
			if idx := lhmSocketIndex(id); idx >= 0 && lhmHasMultipleSockets(lhmM, hw) {
				prefix = fmt.Sprintf("CPU%d: ", idx)
			}
			out = append(out, TempEntry{prefix + name + " (" + hw + ")", m.Value})
		}
	}

	if len(out) == 0 {
		for _, m := range cpuTempM {
			if m.Name == "cpu_temp_celsius" {
				out = append(out, TempEntry{m.Labels["zone"], m.Value})
			}
		}
	}

	seen := map[string]bool{}
	for _, m := range nvmeM {
		if m.Name == "nvme_temperature_celsius" {
			dev := m.Labels["device"]
			if !seen[dev] {
				seen[dev] = true
				out = append(out, TempEntry{"NVMe " + dev, m.Value})
			}
		}
	}

	for _, m := range smartM {
		if m.Name == "smart_temp_celsius" {
			out = append(out, TempEntry{"SATA " + m.Labels["device"], m.Value})
		}
	}

	for _, m := range ipmiM {
		if m.Name == "ipmi_temperature_celsius" {
			out = append(out, TempEntry{"BMC " + ipmiSensorName(m.Labels["sensor"]), m.Value})
		}
	}

	return out
}

func buildVoltages(sensorsM, lhmM, ipmiM []collectors.Metric) []VoltageEntry {
	var out []VoltageEntry
	for _, m := range sensorsM {
		if m.Name == "sensor_voltage_volts" {
			out = append(out, VoltageEntry{m.Labels["name"], m.Value})
		}
	}
	for _, m := range lhmM {
		if m.Name == "lhm_voltage_volts" && !strings.HasPrefix(m.Labels["name"], "CPU Core #") {
			prefix := ""
			if idx := lhmSocketIndex(m.Labels["identifier"]); idx >= 0 && lhmHasMultipleSockets(lhmM, m.Labels["hardware"]) {
				prefix = fmt.Sprintf("CPU%d: ", idx)
			}
			out = append(out, VoltageEntry{prefix + m.Labels["name"], m.Value})
		}
	}
	for _, m := range ipmiM {
		if m.Name == "ipmi_voltage_volts" {
			out = append(out, VoltageEntry{"BMC " + m.Labels["sensor"], m.Value})
		}
	}
	return out
}

func buildFans(sensorsM, lhmM, ipmiM []collectors.Metric) []FanEntry {
	var out []FanEntry
	for _, m := range sensorsM {
		if m.Name == "sensor_fan_rpm" && m.Value > 0 {
			out = append(out, FanEntry{m.Labels["name"], m.Value})
		}
	}
	for _, m := range lhmM {
		if m.Name == "lhm_fan_rpm" && m.Value > 0 {
			out = append(out, FanEntry{m.Labels["name"], m.Value})
		}
	}
	for _, m := range ipmiM {
		if m.Name == "ipmi_fan_rpm" && m.Value > 0 {
			out = append(out, FanEntry{"BMC " + m.Labels["sensor"], m.Value})
		}
	}
	return out
}

func buildDisk(m []collectors.Metric) ([]DiskUsage, []DiskIO) {
	usageMap := map[string]*DiskUsage{}
	ioMap := map[string]*DiskIO{}
	for _, metric := range m {
		mp := metric.Labels["mountpoint"]
		dev := metric.Labels["device"]
		if mp != "" {
			if _, ok := usageMap[mp]; !ok {
				usageMap[mp] = &DiskUsage{Mountpoint: mp, Device: dev}
			}
			u := usageMap[mp]
			switch metric.Name {
			case "disk_total_bytes":
				u.TotalBytes = metric.Value
			case "disk_used_bytes":
				u.UsedBytes = metric.Value
			case "disk_free_bytes":
				u.FreeBytes = metric.Value
			case "disk_usage_percent":
				u.UsagePercent = metric.Value
			}
		} else if dev != "" {
			if _, ok := ioMap[dev]; !ok {
				ioMap[dev] = &DiskIO{Device: dev}
			}
			io := ioMap[dev]
			switch metric.Name {
			case "disk_read_bytes_total":
				io.ReadBytes = metric.Value
			case "disk_write_bytes_total":
				io.WriteBytes = metric.Value
			}
		}
	}
	var usage []DiskUsage
	for _, u := range usageMap {
		usage = append(usage, *u)
	}
	var ios []DiskIO
	for _, io := range ioMap {
		ios = append(ios, *io)
	}
	return usage, ios
}

func buildNVMe(m []collectors.Metric) []NVMeEntry {
	devMap := map[string]*NVMeEntry{}
	for _, metric := range m {
		dev := metric.Labels["device"]
		if _, ok := devMap[dev]; !ok {
			devMap[dev] = &NVMeEntry{Device: dev}
		}
		e := devMap[dev]
		switch metric.Name {
		case "nvme_percentage_used":
			e.LifeRemaining = 100 - metric.Value
		case "nvme_available_spare_percent":
			e.SpareAvail = metric.Value
			e.HasSpare = true
		case "nvme_power_on_hours":
			e.PowerOnHours = metric.Value
		case "nvme_media_errors_total":
			e.MediaErrors = metric.Value
		case "nvme_temperature_celsius":
			e.TempC = metric.Value
			e.HasTemp = true
		}
	}
	var out []NVMeEntry
	for _, e := range devMap {
		out = append(out, *e)
	}
	return out
}

func buildSATA(m []collectors.Metric) []SATAEntry {
	devMap := map[string]*SATAEntry{}
	for _, metric := range m {
		dev := metric.Labels["device"]
		if _, ok := devMap[dev]; !ok {
			devMap[dev] = &SATAEntry{Device: dev}
		}
		e := devMap[dev]
		switch metric.Name {
		case "smart_life_remaining_percent":
			e.LifeRemaining = metric.Value
			e.HasLife = true
		case "smart_spare_available_percent":
			e.SpareAvail = metric.Value
			e.HasSpare = true
		case "smart_power_on_hours":
			e.PowerOnHours = metric.Value
			e.HasHours = true
		case "smart_reallocated_sectors":
			e.Reallocated = metric.Value
			e.HasReallocated = true
		case "smart_pending_sectors":
			e.Pending = metric.Value
			e.HasPending = true
		case "smart_temp_celsius":
			e.TempC = metric.Value
			e.HasTemp = true
		}
	}
	var out []SATAEntry
	for _, e := range devMap {
		out = append(out, *e)
	}
	return out
}

func buildNetwork(m []collectors.Metric) []NetworkEntry {
	ifMap := map[string]*NetworkEntry{}
	for _, metric := range m {
		iface := metric.Labels["interface"]
		if _, ok := ifMap[iface]; !ok {
			ifMap[iface] = &NetworkEntry{Interface: iface}
		}
		e := ifMap[iface]
		switch metric.Name {
		case "network_bytes_sent_total":
			e.SentBytes = metric.Value
		case "network_bytes_recv_total":
			e.RecvBytes = metric.Value
		}
	}
	var out []NetworkEntry
	for _, e := range ifMap {
		if e.SentBytes > 0 || e.RecvBytes > 0 {
			out = append(out, *e)
		}
	}
	return out
}

// ── Helpers (mirrors tui.go) ─────────────────────────────────────────────────

func lhmSocketIndex(identifier string) int {
	parts := strings.Split(strings.TrimPrefix(identifier, "/"), "/")
	if len(parts) >= 2 {
		idx := 0
		for _, c := range parts[1] {
			if c >= '0' && c <= '9' {
				idx = idx*10 + int(c-'0')
			} else {
				return -1
			}
		}
		return idx
	}
	return -1
}

func lhmHasMultipleSockets(lhmM []collectors.Metric, hardware string) bool {
	sockets := map[int]bool{}
	for _, m := range lhmM {
		if m.Name == "lhm_temperature_celsius" && m.Labels["hardware"] == hardware {
			if idx := lhmSocketIndex(m.Labels["identifier"]); idx >= 0 {
				sockets[idx] = true
			}
		}
	}
	return len(sockets) > 1
}

func ipmiSensorName(raw string) string {
	switch raw {
	case "Sys_Temp1":
		return "Inlet"
	case "Sys_Temp2":
		return "Exhaust"
	case "CPU0_Temp":
		return "CPU0"
	case "CPU1_Temp":
		return "CPU1"
	default:
		return raw
	}
}
