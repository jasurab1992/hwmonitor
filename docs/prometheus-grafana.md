# HWmonitor — Prometheus + Grafana

## Запуск экспортера

### Требования
- Windows (x64), запуск от **Администратора** (необходим для Ring0, IPMI, SMART)
- Порт `9100` открыт на сервере (или любой другой из config.yaml)

### Быстрый старт

```cmd
hwmonitor.exe --mode exporter
```

Метрики доступны по адресу:
```
http://<ip-сервера>:9100/metrics
```

### Одновременно TUI + экспортер

```cmd
hwmonitor.exe --mode both
```

### Кастомный конфиг

```cmd
hwmonitor.exe --mode exporter --config C:\hwmonitor\config.yaml
```

---

## config.yaml

```yaml
prometheus_port: 9100       # порт HTTP-сервера
collect_interval: 5s        # интервал сбора для TUI-режима

collectors:
  cpu:      true   # загрузка CPU (total + per-core)
  memory:   true   # RAM и Swap
  disk:     true   # использование разделов + I/O счётчики
  nvme:     true   # NVMe SMART (ресурс, ошибки, температура)
  smart:    true   # SATA SMART (ресурс, bad sectors, температура)
  network:  true   # сетевой трафик по интерфейсам
  sysinfo:  true   # информация о системе (CPU, память, плата)
  sensors:  true   # температуры через WMI (OHM/LHM сенсоры)
  cpu_temp: true   # температура CPU через ACPI / Ring0 (fallback)
  ipmi:     true   # BMC: температуры, вентиляторы, напряжения
  lhm:      true   # LibreHardwareMonitor: подробные CPU/GPU сенсоры
```

---

## Запуск как Windows-служба

Рекомендуется [NSSM](https://nssm.cc/download) (Non-Sucking Service Manager):

```cmd
nssm install HWmonitor "C:\hwmonitor\hwmonitor.exe"
nssm set HWmonitor AppParameters "--mode exporter"
nssm set HWmonitor AppDirectory "C:\hwmonitor"
nssm set HWmonitor ObjectName LocalSystem
nssm set HWmonitor Start SERVICE_AUTO_START
nssm start HWmonitor
```

Либо через встроенный `sc.exe`:

```cmd
sc create HWmonitor binPath= "C:\hwmonitor\hwmonitor.exe --mode exporter" start= auto obj= LocalSystem
sc start HWmonitor
```

> **Важно:** служба должна запускаться от `LocalSystem` или учётной записи с правами администратора.

---

## Настройка Prometheus

Добавьте в `prometheus.yml`:

```yaml
scrape_configs:
  - job_name: 'hwmonitor'
    static_configs:
      - targets:
          - '192.168.1.10:9100'   # адрес сервера
          - '192.168.1.11:9100'   # второй сервер (если есть)
    scrape_interval: 10s
    scrape_timeout:  9s
```

Проверка что Prometheus видит таргет:
```
http://<prometheus>:9090/targets
```

---

## Полный список метрик

Все метрики публикуются с префиксом `hwmonitor_`.

### CPU

| Метрика | Тип | Лейблы | Описание |
|---|---|---|---|
| `hwmonitor_cpu_usage_percent` | Gauge | — | Общая загрузка CPU, % |
| `hwmonitor_cpu_core_usage_percent` | Gauge | `core` | Загрузка по ядрам, % |
| `hwmonitor_cpu_cores_total` | Gauge | — | Кол-во логических ядер |
| `hwmonitor_cpu_frequency_mhz` | Gauge | — | Текущая частота, МГц |

### Память

| Метрика | Тип | Лейблы | Описание |
|---|---|---|---|
| `hwmonitor_memory_total_bytes` | Gauge | — | Всего RAM |
| `hwmonitor_memory_used_bytes` | Gauge | — | Занято |
| `hwmonitor_memory_available_bytes` | Gauge | — | Свободно |
| `hwmonitor_memory_usage_percent` | Gauge | — | Загрузка, % |
| `hwmonitor_swap_total_bytes` | Gauge | — | Всего Swap |
| `hwmonitor_swap_used_bytes` | Gauge | — | Занято Swap |
| `hwmonitor_swap_usage_percent` | Gauge | — | Загрузка Swap, % |

### Диски

| Метрика | Тип | Лейблы | Описание |
|---|---|---|---|
| `hwmonitor_disk_total_bytes` | Gauge | `mountpoint`, `device` | Ёмкость раздела |
| `hwmonitor_disk_used_bytes` | Gauge | `mountpoint`, `device` | Занято |
| `hwmonitor_disk_free_bytes` | Gauge | `mountpoint`, `device` | Свободно |
| `hwmonitor_disk_usage_percent` | Gauge | `mountpoint`, `device` | Заполнение, % |
| `hwmonitor_disk_read_bytes_total` | Counter | `device` | Прочитано байт (накопленно) |
| `hwmonitor_disk_write_bytes_total` | Counter | `device` | Записано байт (накопленно) |
| `hwmonitor_disk_read_count_total` | Counter | `device` | Кол-во операций чтения |
| `hwmonitor_disk_write_count_total` | Counter | `device` | Кол-во операций записи |

### NVMe SMART

| Метрика | Тип | Лейблы | Описание |
|---|---|---|---|
| `hwmonitor_nvme_temperature_celsius` | Gauge | `device` | Температура, °C |
| `hwmonitor_nvme_percentage_used` | Gauge | `device` | Износ, % (0=новый, 100=конец) |
| `hwmonitor_nvme_available_spare_percent` | Gauge | `device` | Доступный резерв, % |
| `hwmonitor_nvme_power_on_hours` | Gauge | `device` | Часов работы |
| `hwmonitor_nvme_media_errors_total` | Counter | `device` | Ошибки медиа |

### SATA SMART

| Метрика | Тип | Лейблы | Описание |
|---|---|---|---|
| `hwmonitor_smart_temp_celsius` | Gauge | `device`, `drive` | Температура, °C |
| `hwmonitor_smart_life_remaining_percent` | Gauge | `device`, `drive` | Остаток ресурса, % |
| `hwmonitor_smart_spare_available_percent` | Gauge | `device`, `drive` | Резервные блоки, % |
| `hwmonitor_smart_power_on_hours` | Gauge | `device`, `drive` | Часов работы |
| `hwmonitor_smart_reallocated_sectors` | Gauge | `device`, `drive` | Переназначенных секторов |
| `hwmonitor_smart_pending_sectors` | Gauge | `device`, `drive` | Нестабильных секторов |

### Сеть

| Метрика | Тип | Лейблы | Описание |
|---|---|---|---|
| `hwmonitor_network_bytes_sent_total` | Counter | `interface` | Отправлено байт |
| `hwmonitor_network_bytes_recv_total` | Counter | `interface` | Получено байт |
| `hwmonitor_network_packets_sent_total` | Counter | `interface` | Отправлено пакетов |
| `hwmonitor_network_packets_recv_total` | Counter | `interface` | Получено пакетов |
| `hwmonitor_network_errors_in_total` | Counter | `interface` | Ошибок входящих |
| `hwmonitor_network_errors_out_total` | Counter | `interface` | Ошибок исходящих |

### IPMI / BMC (если доступен)

| Метрика | Тип | Лейблы | Описание |
|---|---|---|---|
| `hwmonitor_ipmi_temperature_celsius` | Gauge | `sensor` | Температура (Inlet, Exhaust, CPU0...) |
| `hwmonitor_ipmi_fan_rpm` | Gauge | `sensor` | Обороты вентиляторов |
| `hwmonitor_ipmi_voltage_volts` | Gauge | `sensor` | Напряжения (P12V, P5V, PVCCIN...) |

### LHM / LibreHardwareMonitor (если доступен)

| Метрика | Тип | Лейблы | Описание |
|---|---|---|---|
| `hwmonitor_lhm_temperature_celsius` | Gauge | `name`, `hardware`, `hw_type`, `identifier` | CPU Package, Core Max и др. |
| `hwmonitor_lhm_voltage_volts` | Gauge | `name`, `hardware`, `hw_type`, `identifier` | CPU Core и др. |
| `hwmonitor_lhm_fan_rpm` | Gauge | `name`, `hardware`, `hw_type`, `identifier` | Вентиляторы через SuperIO |

---

## Grafana — настройка

### 1. Добавить источник данных Prometheus

**Connections → Data sources → Add → Prometheus**

```
URL: http://<prometheus>:9090
```

### 2. Полезные PromQL запросы для дашборда

#### CPU

```promql
# Общая загрузка по серверам
hwmonitor_cpu_usage_percent

# Загрузка по ядрам (heatmap)
hwmonitor_cpu_core_usage_percent

# Топ-5 загруженных ядер
topk(5, hwmonitor_cpu_core_usage_percent)
```

#### Память

```promql
# Использование RAM в байтах
hwmonitor_memory_used_bytes

# Процент использования
hwmonitor_memory_usage_percent

# Свободная память
hwmonitor_memory_available_bytes
```

#### Диски — I/O скорость (rate от счётчиков)

```promql
# Скорость записи, байт/сек
rate(hwmonitor_disk_write_bytes_total[1m])

# Скорость чтения, байт/сек
rate(hwmonitor_disk_read_bytes_total[1m])

# Заполнение раздела C:
hwmonitor_disk_usage_percent{mountpoint="C:"}
```

#### Сеть — трафик (rate)

```promql
# Входящий трафик, байт/сек
rate(hwmonitor_network_bytes_recv_total[1m])

# Исходящий трафик, байт/сек
rate(hwmonitor_network_bytes_sent_total[1m])
```

#### Температуры

```promql
# Все температуры с IPMI (ambient, CPU, exhaust)
hwmonitor_ipmi_temperature_celsius

# Температура инлета (ambient)
hwmonitor_ipmi_temperature_celsius{sensor="Sys_Temp1"}

# CPU Package температуры через LHM
hwmonitor_lhm_temperature_celsius{name=~"CPU Package"}

# NVMe температуры
hwmonitor_nvme_temperature_celsius
```

#### Вентиляторы и напряжения

```promql
# Обороты всех вентиляторов BMC
hwmonitor_ipmi_fan_rpm

# Напряжение P12V
hwmonitor_ipmi_voltage_volts{sensor="P12V"}

# Все напряжения BMC
hwmonitor_ipmi_voltage_volts
```

#### SMART — алерты

```promql
# Диски с переназначенными секторами > 0
hwmonitor_smart_reallocated_sectors > 0

# NVMe с износом > 80%
hwmonitor_nvme_percentage_used > 80

# SATA с остатком ресурса < 10%
hwmonitor_smart_life_remaining_percent < 10

# NVMe с ошибками медиа
hwmonitor_nvme_media_errors_total > 0
```

### 3. Пример правил алертов (alerting rules)

Добавьте в Prometheus `rules/hwmonitor.yml`:

```yaml
groups:
  - name: hwmonitor
    rules:

      - alert: HighCPUUsage
        expr: hwmonitor_cpu_usage_percent > 90
        for: 5m
        labels:
          severity: warning
        annotations:
          summary: "Высокая загрузка CPU на {{ $labels.instance }}"
          description: "CPU загружен на {{ $value | humanize }}%"

      - alert: LowDiskSpace
        expr: hwmonitor_disk_usage_percent > 85
        for: 1m
        labels:
          severity: warning
        annotations:
          summary: "Мало места на диске {{ $labels.mountpoint }}"
          description: "Заполнено {{ $value | humanize }}%"

      - alert: HighTemperature
        expr: hwmonitor_ipmi_temperature_celsius > 45
        for: 2m
        labels:
          severity: critical
        annotations:
          summary: "Высокая температура {{ $labels.sensor }}"
          description: "{{ $value }}°C"

      - alert: NVMeWearHigh
        expr: hwmonitor_nvme_percentage_used > 80
        labels:
          severity: warning
        annotations:
          summary: "NVMe {{ $labels.device }} изношен на {{ $value }}%"

      - alert: BadSectors
        expr: hwmonitor_smart_reallocated_sectors > 0
        labels:
          severity: critical
        annotations:
          summary: "Переназначенные секторы на {{ $labels.device }}"

      - alert: FanStopped
        expr: hwmonitor_ipmi_fan_rpm == 0
        for: 1m
        labels:
          severity: critical
        annotations:
          summary: "Вентилятор {{ $labels.sensor }} остановлен"
```

Подключите в `prometheus.yml`:

```yaml
rule_files:
  - "rules/hwmonitor.yml"
```

---

## Мультисерверный мониторинг

Для нескольких серверов используйте лейбл `instance` (добавляется Prometheus автоматически из `targets`) или добавьте свои:

```yaml
scrape_configs:
  - job_name: 'hwmonitor'
    static_configs:
      - targets: ['srv-01:9100']
        labels:
          location: 'rack-A'
      - targets: ['srv-02:9100']
        labels:
          location: 'rack-B'
```

Тогда в Grafana можно фильтровать по `instance` или `location`.

---

## Проверка метрик вручную

```powershell
# На сервере
Invoke-RestMethod http://localhost:9100/metrics | Select-String "hwmonitor_cpu"

# Или в браузере
http://localhost:9100/metrics
```
