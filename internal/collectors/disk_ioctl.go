//go:build windows

package collectors

// diskIOCTL provides IOCTL-based drive enumeration, device identification,
// and temperature data. All SMART health data is collected via smartctl.

import (
	"fmt"
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ─── IOCTL codes ─────────────────────────────────────────────────────────────

const ioctlStorageQueryProperty = 0x002D1400

// StoragePropertyId
const (
	storageDeviceProperty            = 0
	storageDeviceTemperatureProperty = 0x12
)

const propertyStandardQuery = 0

// STORAGE_BUS_TYPE
const (
	busTypeNvme = 17
	busTypeSata = 11
	busTypeAta  = 3
	busTypeSas  = 10
)

// ─── Structs ─────────────────────────────────────────────────────────────────

type storagePropertyQuery struct {
	PropertyId           uint32
	QueryType            uint32
	AdditionalParameters [4]byte
}

type storageDeviceDescriptorHdr struct {
	Version               uint32
	Size                  uint32
	DeviceType            byte
	DeviceTypeModifier    byte
	RemovableMedia        byte
	CommandQueueing       byte
	VendorIdOffset        uint32
	ProductIdOffset       uint32
	ProductRevisionOffset uint32
	SerialNumberOffset    uint32
	BusType               byte
	_                     [3]byte
	RawPropertiesLength   uint32
}

type storageTemperatureDescriptorHdr struct {
	Version             uint32
	Size                uint32
	CriticalTemperature int16
	WarningTemperature  int16
	InfoCount           uint16
	_                   [2]byte
	_                   [8]byte
}

type storageTemperatureInfo struct {
	Index          uint16
	Temperature    int16
	OverThreshold  int16
	UnderThreshold int16
	ValidThresholds byte
	_               [1]byte
}

// ─── physicalDriveInfo ───────────────────────────────────────────────────────

// physicalDriveInfo holds all collected data for one physical drive.
// Device identity comes from IOCTL; temperature from IOCTL with smartctl fallback;
// all SMART health data from smartctl.
type physicalDriveInfo struct {
	Index   int
	Model   string
	Serial  string
	BusType int

	// Temperature — IOCTL primary, smartctl fallback
	TempC   float64
	HasTemp bool

	// Endurance / health (NVMe + SATA SSD)
	PercentUsed    int // % endurance consumed (0 = new, 100 = worn)
	HasPercentUsed bool
	SpareAvail     int // available spare %
	HasSpareAvail  bool

	// Time and errors
	PowerOnHours    uint64
	HasPowerOnHours bool
	MediaErrors     uint64
	HasMediaErrors  bool

	// SATA sector health
	ReallocatedSectors uint32
	HasReallocated     bool
	PendingSectors     uint32
	HasPending         bool
}

// ─── Enumeration ─────────────────────────────────────────────────────────────

// EnumeratePhysicalDrives opens \\.\PhysicalDriveN, reads device identity and
// temperature via IOCTL, then collects all SMART health data via smartctl.
func EnumeratePhysicalDrives() []physicalDriveInfo {
	var results []physicalDriveInfo
	consecutive := 0
	for i := 0; i < 32; i++ {
		path, _ := syscall.UTF16PtrFromString(fmt.Sprintf(`\\.\PhysicalDrive%d`, i))
		h, err := windows.CreateFile(
			path,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil, windows.OPEN_EXISTING, 0, 0,
		)
		if err != nil {
			consecutive++
			if consecutive >= 3 {
				break
			}
			continue
		}
		consecutive = 0

		info := physicalDriveInfo{Index: i}
		readDeviceDescriptor(h, &info)
		readTemperature(h, &info)
		windows.CloseHandle(h)

		collectSmartData(&info)
		results = append(results, info)
	}
	return results
}

// ─── Device descriptor ────────────────────────────────────────────────────────

func readDeviceDescriptor(h windows.Handle, info *physicalDriveInfo) {
	q := storagePropertyQuery{PropertyId: storageDeviceProperty, QueryType: propertyStandardQuery}
	buf := make([]byte, 512)
	var returned uint32
	if err := windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		&buf[0], uint32(len(buf)), &returned, nil); err != nil {
		log.Printf("disk: descriptor IOCTL failed for PhysicalDrive%d: %v", info.Index, err)
		return
	}
	if returned < uint32(unsafe.Sizeof(storageDeviceDescriptorHdr{})) {
		return
	}

	hdr := (*storageDeviceDescriptorHdr)(unsafe.Pointer(&buf[0]))
	info.BusType = int(hdr.BusType)

	str := func(off uint32) string {
		if off == 0 || off >= uint32(len(buf)) {
			return ""
		}
		end := off
		for end < uint32(len(buf)) && buf[end] != 0 {
			end++
		}
		return trimSpace(string(buf[off:end]))
	}

	vendor := str(hdr.VendorIdOffset)
	product := str(hdr.ProductIdOffset)
	if vendor != "" {
		info.Model = vendor + " " + product
	} else {
		info.Model = product
	}
	info.Serial = str(hdr.SerialNumberOffset)
}

// ─── Temperature ─────────────────────────────────────────────────────────────

func readTemperature(h windows.Handle, info *physicalDriveInfo) {
	const hdrSize = int(unsafe.Sizeof(storageTemperatureDescriptorHdr{}))
	const infoSize = int(unsafe.Sizeof(storageTemperatureInfo{}))

	q := storagePropertyQuery{PropertyId: storageDeviceTemperatureProperty, QueryType: propertyStandardQuery}
	buf := make([]byte, hdrSize+infoSize*8)
	var returned uint32
	if err := windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		&buf[0], uint32(len(buf)), &returned, nil); err != nil {
		return
	}
	if returned < uint32(hdrSize) {
		return
	}

	hdr := (*storageTemperatureDescriptorHdr)(unsafe.Pointer(&buf[0]))
	if hdr.InfoCount == 0 {
		return
	}

	entry := (*storageTemperatureInfo)(unsafe.Pointer(&buf[hdrSize]))
	raw := float64(entry.Temperature)
	var celsius float64
	if raw > 2000 { // Kelvin*10
		celsius = raw/10.0 - 273.15
	} else {
		celsius = raw / 10.0
	}
	if celsius > -20 && celsius < 120 {
		info.TempC = celsius
		info.HasTemp = true
	}
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && (s[start] == ' ' || s[start] == '\t') {
		start++
	}
	for end > start && (s[end-1] == ' ' || s[end-1] == '\t') {
		end--
	}
	return s[start:end]
}
