# HWmonitor

Windows hardware monitor with a terminal UI, desktop GUI, and Prometheus exporter.

Collects CPU usage & temperature, memory, disk I/O, NVMe/SATA SMART, network traffic, BMC/IPMI sensors, and LibreHardwareMonitor data.

---

## Modes

| Flag | Description |
|---|---|
| `hwmonitor.exe` | Terminal UI (TUI) |
| `hwmonitor.exe --mode exporter` | Prometheus HTTP exporter only |
| `hwmonitor.exe --mode both` | TUI + exporter simultaneously |
| `hwmonitorui.exe` | Desktop GUI (Wails) |

> **Requires Administrator** — Ring0 driver access, IPMI, and SMART data need elevated privileges.

---

## Quick start

```cmd
hwmonitor.exe
```

Prometheus exporter on port `9100`:

```cmd
hwmonitor.exe --mode exporter
```

Custom config:

```cmd
hwmonitor.exe --mode exporter --config C:\hwmonitor\config.yaml
```

---

## config.yaml

```yaml
prometheus_port: 9100       # HTTP port for /metrics
collect_interval: 5s        # TUI refresh interval

collectors:
  cpu:      true
  memory:   true
  disk:     true
  nvme:     true
  smart:    true
  network:  true
  sysinfo:  true
  sensors:  true
  cpu_temp: true
  ipmi:     true
  lhm:      true
```

---

## Embedded tools

The following binaries can be compiled into the executable with build tags:

| Tag | Tool | Purpose |
|---|---|---|
| `embed_smartctl` | smartctl.exe | NVMe/SATA SMART data |
| `embed_ipmiutil` | ipmiutil.exe + DLLs | BMC sensor data (temps, fans, voltages) |
| `embed_lhm` | lhm_bridge.exe | LibreHardwareMonitor (CPU Package, GPU, SuperIO) |

Build with all tools embedded:

```cmd
go build -tags "embed_smartctl,embed_ipmiutil,embed_lhm" -o hwmonitor.exe .
```

Without tags the tools are looked up from PATH or common install locations.

---

## Desktop UI (hwmonitorui)

Wails v2 desktop application. Build:

```cmd
cd hwmonitorui
wails build -tags "embed_smartctl,embed_ipmiutil,embed_lhm"
```

Tabs: **Resources** (60-second scrolling charts), **Hardware** (CPU, Memory, Temperatures, Fans, Voltages), **Storage** (Disk, NVMe, SATA SMART).

---

## Windows Service

Using [NSSM](https://nssm.cc/download):

```cmd
nssm install HWmonitor "C:\hwmonitor\hwmonitor.exe"
nssm set HWmonitor AppParameters "--mode exporter"
nssm set HWmonitor AppDirectory "C:\hwmonitor"
nssm set HWmonitor ObjectName LocalSystem
nssm set HWmonitor Start SERVICE_AUTO_START
nssm start HWmonitor
```

Or with `sc.exe`:

```cmd
sc create HWmonitor binPath= "C:\hwmonitor\hwmonitor.exe --mode exporter" start= auto obj= LocalSystem
sc start HWmonitor
```

---

## Prometheus + Grafana

See **[docs/prometheus-grafana.md](docs/prometheus-grafana.md)** for:

- Prometheus scrape config
- Full metrics reference (CPU, Memory, Disk, NVMe, SATA, Network, IPMI, LHM)
- PromQL example queries
- Grafana dashboard setup
- Alert rules (HighCPU, LowDisk, HighTemp, NVMeWear, BadSectors, FanStopped)
- Multi-server monitoring

---

## Collected metrics

| Category | Metrics |
|---|---|
| CPU | Usage %, per-core %, frequency, core count |
| Memory | RAM total/used/free/%, Swap total/used/% |
| Disk | Partition usage, I/O bytes/ops counters |
| NVMe SMART | Temperature, wear %, spare %, power-on hours, media errors |
| SATA SMART | Temperature, life remaining, spare %, power-on hours, reallocated/pending sectors |
| Network | Bytes/packets sent/received, errors |
| IPMI/BMC | Temperatures (Inlet, Exhaust, CPU), fan RPM, voltages |
| LHM | CPU Package temp, core voltages, GPU, SuperIO fans |
| System Info | CPU model, board, RAM slots (type, speed, size) |
