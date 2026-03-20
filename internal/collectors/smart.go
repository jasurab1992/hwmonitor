//go:build windows

package collectors

import (
	"fmt"
	"log"
	"unsafe"

	"golang.org/x/sys/windows"
)

const (
	ioctlATAPassThroughDirect = 0x4D028
	ataSmartCmd               = 0xB0
	smartReadData             = 0xD0
)

// SMART attribute IDs we care about
const (
	smartAttrReallocatedSectors = 5
	smartAttrPowerOnHours       = 9
	smartAttrSpinRetries        = 10
	smartAttrTemperature190     = 190
	smartAttrTemperature194     = 194
	smartAttrPendingSectors     = 197
)

// ataPassThroughDirect mirrors the Windows ATA_PASS_THROUGH_DIRECT structure.
type ataPassThroughDirect struct {
	Length             uint16
	AtaFlags           uint16
	PathId             uint8
	TargetId           uint8
	Lun                uint8
	ReservedAsUchar    uint8
	DataTransferLength uint32
	TimeOutValue       uint32
	ReservedAsUlong    uint32
	DataBuffer         uintptr
	PreviousTaskFile   [8]uint8
	CurrentTaskFile    [8]uint8
}

const (
	ataFlagsDataIn   = 0x02
	ataFlagsDrdy     = 0x08
)

// smartAttribute represents a single SMART attribute from the data buffer.
type smartAttribute struct {
	ID         uint8
	Flags      uint16
	Current    uint8
	Worst      uint8
	RawValue   [6]uint8
	Reserved   uint8
}

// SMARTCollector collects ATA SMART attributes from physical drives.
type SMARTCollector struct{}

func NewSMARTCollector() *SMARTCollector {
	return &SMARTCollector{}
}

func (s *SMARTCollector) Name() string {
	return "smart"
}

func (s *SMARTCollector) Collect() ([]Metric, error) {
	var metrics []Metric

	for i := 0; i < 16; i++ {
		drivePath := fmt.Sprintf(`\\.\PhysicalDrive%d`, i)
		m, err := collectSMARTDrive(drivePath)
		if err != nil {
			continue
		}
		metrics = append(metrics, m...)
	}

	if len(metrics) == 0 {
		log.Printf("smart: no ATA/SATA drives found or accessible (requires admin rights)")
	}

	return metrics, nil
}

func collectSMARTDrive(drivePath string) ([]Metric, error) {
	pathPtr, err := windows.UTF16PtrFromString(drivePath)
	if err != nil {
		return nil, err
	}

	handle, err := windows.CreateFile(
		pathPtr,
		windows.GENERIC_READ|windows.GENERIC_WRITE,
		windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
		nil,
		windows.OPEN_EXISTING,
		0,
		0,
	)
	if err != nil {
		return nil, err
	}
	defer windows.CloseHandle(handle)

	// Prepare the data buffer for SMART READ DATA (512 bytes)
	dataBuf := make([]byte, 512)

	apt := ataPassThroughDirect{
		Length:             uint16(unsafe.Sizeof(ataPassThroughDirect{})),
		AtaFlags:           ataFlagsDataIn | ataFlagsDrdy,
		DataTransferLength: 512,
		TimeOutValue:       5,
		DataBuffer:         uintptr(unsafe.Pointer(&dataBuf[0])),
	}

	// ATA command: SMART READ DATA
	// CurrentTaskFile layout: [Features, SectorCount, LbaLow, LbaMid, LbaHigh, Device, Command, Reserved]
	apt.CurrentTaskFile[0] = smartReadData  // Features = SMART_READ_DATA
	apt.CurrentTaskFile[1] = 1             // Sector count
	apt.CurrentTaskFile[3] = 0x4F          // LbaMid = 0x4F (SMART signature)
	apt.CurrentTaskFile[4] = 0xC2          // LbaHigh = 0xC2 (SMART signature)
	apt.CurrentTaskFile[6] = ataSmartCmd   // Command = 0xB0

	var bytesReturned uint32
	err = windows.DeviceIoControl(
		handle,
		ioctlATAPassThroughDirect,
		(*byte)(unsafe.Pointer(&apt)),
		uint32(unsafe.Sizeof(apt)),
		(*byte)(unsafe.Pointer(&apt)),
		uint32(unsafe.Sizeof(apt)),
		&bytesReturned,
		nil,
	)
	if err != nil {
		return nil, fmt.Errorf("SMART IOCTL failed on %s: %w", drivePath, err)
	}

	// Parse SMART attributes from the 512-byte data buffer.
	// Attributes start at offset 2 and each attribute is 12 bytes.
	// There are up to 30 attributes.
	var metrics []Metric
	temperatureFound := false

	for j := 0; j < 30; j++ {
		offset := 2 + j*12
		if offset+12 > len(dataBuf) {
			break
		}

		attrID := dataBuf[offset]
		if attrID == 0 {
			continue
		}

		rawLow := uint64(dataBuf[offset+5]) | uint64(dataBuf[offset+6])<<8 |
			uint64(dataBuf[offset+7])<<16 | uint64(dataBuf[offset+8])<<24

		switch attrID {
		case smartAttrTemperature194, smartAttrTemperature190:
			if !temperatureFound {
				metrics = append(metrics, Metric{
					Name:   "smart_temp_celsius",
					Value:  float64(rawLow & 0xFF),
					Labels: map[string]string{"drive": drivePath},
				})
				temperatureFound = true
			}
		case smartAttrReallocatedSectors:
			metrics = append(metrics, Metric{
				Name:   "smart_reallocated_sectors",
				Value:  float64(rawLow),
				Labels: map[string]string{"drive": drivePath},
			})
		case smartAttrPendingSectors:
			metrics = append(metrics, Metric{
				Name:   "smart_pending_sectors",
				Value:  float64(rawLow),
				Labels: map[string]string{"drive": drivePath},
			})
		case smartAttrPowerOnHours:
			metrics = append(metrics, Metric{
				Name:   "smart_power_on_hours",
				Value:  float64(rawLow),
				Labels: map[string]string{"drive": drivePath},
			})
		case smartAttrSpinRetries:
			metrics = append(metrics, Metric{
				Name:   "smart_spin_retries",
				Value:  float64(rawLow),
				Labels: map[string]string{"drive": drivePath},
			})
		}
	}

	return metrics, nil
}
