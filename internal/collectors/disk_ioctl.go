//go:build windows

package collectors

// diskIOCTL provides IOCTL-based disk SMART and temperature data.
// Uses IOCTL_STORAGE_QUERY_PROPERTY (Windows 10+) to read:
//   - Basic device descriptor (bus type, model, serial)
//   - Temperature (StorageDeviceTemperatureProperty)
//   - NVMe SMART via StorageDeviceProtocolSpecificProperty (log page 0x02)
//   - SATA SMART via ATA pass-through

import (
	"encoding/binary"
	"fmt"
	"log"
	"syscall"
	"unsafe"

	"golang.org/x/sys/windows"
)

// ─── IOCTL codes ─────────────────────────────────────────────────────────────

const (
	// IOCTL_STORAGE_QUERY_PROPERTY = CTL_CODE(0x2d, 0x500, METHOD_BUFFERED=0, FILE_ANY_ACCESS=0)
	// = (0x2d << 16) | (0 << 14) | (0x500 << 2) | 0 = 0x002D1400
	ioctlStorageQueryProperty = 0x002D1400

	// IOCTL_ATA_PASS_THROUGH = CTL_CODE(IOCTL_SCSI_BASE=4, 0x40B, METHOD_BUFFERED=0, FILE_READ|FILE_WRITE=3)
	// = (4 << 16) | (3 << 14) | (0x40B << 2) | 0 = 0x40000 | 0xC000 | 0x102C = 0x0004D02C
	ioctlAtaPassThrough = 0x0004D02C
)

// StoragePropertyId
const (
	storageDeviceProperty                = 0
	storageDeviceTemperatureProperty     = 0x12
	storageDeviceProtocolSpecificProperty = 0x13
)

// StorageQueryType
const propertyStandardQuery = 0

// ProtocolType for StorageDeviceProtocolSpecificProperty
const (
	protocolTypeAta  = 2
	protocolTypeNvme = 3
)

// NVMe Data Type
const (
	nvmeDataTypeLogPage = 2
)

// NVMe Log Page IDs
const nvmeLogPageHealthInfo = 0x02

// STORAGE_BUS_TYPE
const (
	busTypeNvme = 17
	busTypeSata = 11
	busTypeAta  = 3
	busTypeSas  = 10
)

// ─── Structs (packed, Windows ABI) ───────────────────────────────────────────

// storagePropertyQuery matches Windows STORAGE_PROPERTY_QUERY.
// In C: DWORD PropertyId + DWORD QueryType + BYTE AdditionalParameters[1] + 3 bytes padding = 12 bytes.
// We use [4]byte to hit the same 12-byte sizeof without the flexible-array complication.
// For protocol-specific queries, nvmeQueryInput is used instead (inlines the extra data at offset 8).
type storagePropertyQuery struct {
	PropertyId           uint32
	QueryType            uint32
	AdditionalParameters [4]byte // 12 bytes total == sizeof(STORAGE_PROPERTY_QUERY) in C
}

// Fixed-size header returned for every StorageDeviceProperty query.
// We read into a large buffer, then interpret the offset fields.
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
	_                     [3]byte // padding
	RawPropertiesLength   uint32
}

// storageTemperatureDataDescriptor header (followed by STORAGE_TEMPERATURE_INFO array).
type storageTemperatureDescriptorHdr struct {
	Version             uint32
	Size                uint32
	CriticalTemperature int16
	WarningTemperature  int16
	InfoCount           uint16
	_                   [2]byte
	_                   [8]byte
}

// storageTemperatureInfo — one entry per sensor.
type storageTemperatureInfo struct {
	Index           uint16
	Temperature     int16 // in tenths of a degree Celsius
	OverThreshold   int16
	UnderThreshold  int16
	ValidThresholds byte
	_               [1]byte
}

// storageProtocolSpecificQueryInput — additional parameters for NVMe log page queries.
type storageProtocolSpecificData struct {
	ProtocolType                uint32
	DataType                    uint32
	ProtocolDataRequestValue    uint32 // log page id for NVMe
	ProtocolDataRequestSubValue uint32
	ProtocolDataOffset          uint32
	ProtocolDataLength          uint32
	FixedProtocolReturnData     uint32
	Reserved                    [3]uint32
}

// NVMe SMART/Health Information log (512 bytes).
type nvmeSmartHealthInfo struct {
	CriticalWarning                    byte
	CompositeTemperature               [2]byte // Kelvin, 0.5K units? No — just uint16 Kelvin
	AvailableSpare                     byte
	AvailableSpareThreshold            byte
	PercentageUsed                     byte
	EnduranceGroupCriticalWarningSummary byte
	_                                  [25]byte
	DataUnitsRead                      [16]byte // 128-bit
	DataUnitsWritten                   [16]byte
	HostReadCommands                   [16]byte
	HostWriteCommands                  [16]byte
	ControllerBusyTime                 [16]byte
	PowerCycles                        [16]byte
	PowerOnHours                       [16]byte
	UnsafeShutdowns                    [16]byte
	MediaErrors                        [16]byte
	NumberOfErrorLogEntries            [16]byte
	WarningCompositeTemperatureTime    uint32
	CriticalCompositeTemperatureTime   uint32
	TemperatureSensor                  [8]uint16
	_                                  [296]byte
}

// ─── ATA PASS-THROUGH ────────────────────────────────────────────────────────

type ataPassThroughEx struct {
	Length             uint16
	AtaFlags           uint16
	PathId             byte
	TargetId           byte
	Lun                byte
	ReservedAsUchar    byte
	DataTransferLength uint32
	TimeOutValue       uint32
	ReservedAsUlong    uint32
	DataBufferOffset   uintptr
	PreviousTaskFile   [8]byte
	CurrentTaskFile    [8]byte
}

const (
	ataFlagsDataIn = 0x02 // ATA_FLAGS_DATA_IN — transfer from device to system (read)
	ataSmartCmd    = 0xB0 // ATA_SMART_CMD
	ataSmartReadData = 0xD0 // SMART_READ_DATA feature
	ataSmartLba1   = 0x4F // SMART LBA mid signature
	ataSmartLba2   = 0xC2 // SMART LBA high signature
)

// ─── PhysicalDriveInfo — result of enumerating one drive ─────────────────────

type physicalDriveInfo struct {
	Index      int
	Model      string
	Serial     string
	BusType    int
	TempC      float64
	HasTemp    bool
	// NVMe SMART
	NVMePercentUsed byte
	NVMePowerOnHours uint64
	NVMeMediaErrors  uint64
	NVMeHasData      bool
	// SATA SMART attributes map: attribute id → value
	SmartAttrs map[byte]smartAttr
}

type smartAttr struct {
	Id            byte
	Value         byte   // normalized 0-255
	Worst         byte
	RawLo         uint32
	RawHi         uint16
}

// ─── Public entry points ──────────────────────────────────────────────────────

// EnumeratePhysicalDrives opens \\.\PhysicalDriveN for N=0..15
// and collects available data from each.
func EnumeratePhysicalDrives() []physicalDriveInfo {
	var results []physicalDriveInfo
	for i := 0; i < 16; i++ {
		path, _ := syscall.UTF16PtrFromString(fmt.Sprintf(`\\.\PhysicalDrive%d`, i))
		h, err := windows.CreateFile(
			path,
			windows.GENERIC_READ|windows.GENERIC_WRITE,
			windows.FILE_SHARE_READ|windows.FILE_SHARE_WRITE,
			nil,
			windows.OPEN_EXISTING,
			0,
			0,
		)
		if err != nil {
			break // no more drives
		}
		info := queryDrive(h, i)
		windows.CloseHandle(h)
		results = append(results, info)
	}
	return results
}

func queryDrive(h windows.Handle, index int) physicalDriveInfo {
	info := physicalDriveInfo{Index: index, SmartAttrs: make(map[byte]smartAttr)}

	// 1. Basic descriptor (model, serial, bus type)
	readDeviceDescriptor(h, &info)

	// 2. Temperature
	readTemperature(h, &info)

	// 3. SMART / health
	switch info.BusType {
	case busTypeNvme:
		readNVMeSmart(h, &info)
	case busTypeSata, busTypeAta:
		readSATASmart(h, &info)
	default:
		// Try ATA SMART for unknown bus types (USB bridges etc.)
		readSATASmart(h, &info)
	}

	return info
}

// ─── Device descriptor ────────────────────────────────────────────────────────

func readDeviceDescriptor(h windows.Handle, info *physicalDriveInfo) {
	q := storagePropertyQuery{PropertyId: storageDeviceProperty, QueryType: propertyStandardQuery}
	buf := make([]byte, 512)
	var returned uint32
	err := windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		&buf[0], uint32(len(buf)),
		&returned, nil)
	if err != nil {
		log.Printf("disk: device descriptor IOCTL failed for PhysicalDrive%d: %v", info.Index, err)
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
	err := windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		&buf[0], uint32(len(buf)),
		&returned, nil)
	if err != nil {
		return // StorageDeviceTemperatureProperty not supported by this drive/driver
	}
	if returned < uint32(hdrSize) {
		return
	}

	hdr := (*storageTemperatureDescriptorHdr)(unsafe.Pointer(&buf[0]))
	if hdr.InfoCount == 0 {
		return
	}

	entry := (*storageTemperatureInfo)(unsafe.Pointer(&buf[hdrSize]))
	// Temperature is in tenths of a degree Celsius on Windows
	// But some drivers report in Kelvin × 10... check for sane range:
	raw := float64(entry.Temperature)
	var celsius float64
	if raw > 2000 { // looks like Kelvin*10
		celsius = raw/10.0 - 273.15
	} else {
		celsius = raw / 10.0
	}
	if celsius > -20 && celsius < 120 {
		info.TempC = celsius
		info.HasTemp = true
	}
}

// ─── NVMe SMART ──────────────────────────────────────────────────────────────

// nvmeQueryInput is the contiguous input buffer for IOCTL_STORAGE_QUERY_PROPERTY
// with StorageDeviceProtocolSpecificProperty.
// Layout: STORAGE_PROPERTY_QUERY header (8B) immediately followed by
// STORAGE_PROTOCOL_SPECIFIC_DATA (40B) — no padding between them.
type nvmeQueryInput struct {
	// STORAGE_PROPERTY_QUERY fields:
	PropertyId uint32
	QueryType  uint32
	// AdditionalParameters[0] == start of STORAGE_PROTOCOL_SPECIFIC_DATA:
	ProtocolType                uint32
	DataType                    uint32
	ProtocolDataRequestValue    uint32
	ProtocolDataRequestSubValue uint32
	ProtocolDataOffset          uint32 // offset from start of this struct's prot section to data in output
	ProtocolDataLength          uint32
	FixedProtocolReturnData     uint32
	Reserved                    [3]uint32
}

func readNVMeSmart(h windows.Handle, info *physicalDriveInfo) {
	const smartSize = 512 // NVMe SMART/Health log page 0x02 is always 512 bytes

	// ProtocolDataOffset is the number of bytes from the start of the
	// STORAGE_PROTOCOL_SPECIFIC_DATA portion to where the payload will appear
	// in the output buffer. That equals sizeof(STORAGE_PROTOCOL_SPECIFIC_DATA) = 40.
	const protSectionSize = 40 // sizeof STORAGE_PROTOCOL_SPECIFIC_DATA

	q := nvmeQueryInput{
		PropertyId:               storageDeviceProtocolSpecificProperty,
		QueryType:                propertyStandardQuery,
		ProtocolType:             protocolTypeNvme,
		DataType:                 nvmeDataTypeLogPage,
		ProtocolDataRequestValue: nvmeLogPageHealthInfo,
		ProtocolDataOffset:       protSectionSize,
		ProtocolDataLength:       smartSize,
	}

	// Output buffer: same header + 512 bytes of NVMe SMART data.
	outBuf := make([]byte, int(unsafe.Sizeof(q))+smartSize)
	var returned uint32
	err := windows.DeviceIoControl(h, ioctlStorageQueryProperty,
		(*byte)(unsafe.Pointer(&q)), uint32(unsafe.Sizeof(q)),
		&outBuf[0], uint32(len(outBuf)),
		&returned, nil)
	if err != nil {
		return // StorageDeviceProtocolSpecificProperty not supported by this NVMe drive/driver
	}

	// NVMe SMART data starts immediately after the query header in the output.
	offset := int(unsafe.Sizeof(q))
	if int(returned) < offset+512 {
		log.Printf("disk: NVMe SMART response too short (%d bytes) for PhysicalDrive%d", returned, info.Index)
		return
	}

	smart := (*nvmeSmartHealthInfo)(unsafe.Pointer(&outBuf[offset]))

	// CompositeTemperature: 16-bit LE, Kelvin
	tempK := uint16(smart.CompositeTemperature[0]) | uint16(smart.CompositeTemperature[1])<<8
	if tempK > 273 {
		info.TempC = float64(tempK) - 273.15
		info.HasTemp = true
	}

	info.NVMePercentUsed = smart.PercentageUsed
	info.NVMePowerOnHours = binary.LittleEndian.Uint64(smart.PowerOnHours[:8])
	info.NVMeMediaErrors = binary.LittleEndian.Uint64(smart.MediaErrors[:8])
	info.NVMeHasData = true
}

// ─── SATA/ATA SMART ──────────────────────────────────────────────────────────

func readSATASmart(h windows.Handle, info *physicalDriveInfo) {
	// ATA PASS-THROUGH: SMART READ DATA (command B0h, feature D0h)
	const dataSize = 512
	type ataBuf struct {
		apt  ataPassThroughEx
		data [dataSize]byte
	}

	buf := ataBuf{}
	buf.apt.Length = uint16(unsafe.Sizeof(ataPassThroughEx{}))
	buf.apt.AtaFlags = ataFlagsDataIn
	buf.apt.DataTransferLength = dataSize
	buf.apt.TimeOutValue = 10
	buf.apt.DataBufferOffset = unsafe.Sizeof(ataPassThroughEx{})
	// ATA_PASS_THROUGH_EX CurrentTaskFile register layout:
	// [0]=Features [1]=SectorCount [2]=LBALow [3]=LBAMid [4]=LBAHigh [5]=Device [6]=Command [7]=Reserved
	buf.apt.CurrentTaskFile[0] = ataSmartReadData // Features = 0xD0 (SMART Read Data)
	buf.apt.CurrentTaskFile[1] = 1                // Sector Count = 1
	buf.apt.CurrentTaskFile[2] = 0                // LBA Low = 0
	buf.apt.CurrentTaskFile[3] = ataSmartLba1     // LBA Mid = 0x4F (SMART signature)
	buf.apt.CurrentTaskFile[4] = ataSmartLba2     // LBA High = 0xC2 (SMART signature)
	buf.apt.CurrentTaskFile[5] = 0xA0             // Device
	buf.apt.CurrentTaskFile[6] = ataSmartCmd      // Command = 0xB0

	var returned uint32
	err := windows.DeviceIoControl(h, ioctlAtaPassThrough,
		(*byte)(unsafe.Pointer(&buf)), uint32(unsafe.Sizeof(buf)),
		(*byte)(unsafe.Pointer(&buf)), uint32(unsafe.Sizeof(buf)),
		&returned, nil)
	if err != nil {
		log.Printf("disk: ATA SMART IOCTL failed for PhysicalDrive%d: %v", info.Index, err)
		return
	}

	// Parse SMART attribute table: starts at offset 2 in the 512-byte data.
	// Each entry is 12 bytes: ID(1), Flags(2), Value(1), Worst(1), Raw[6], Reserved(1)
	data := buf.data[:]
	for i := 2; i+12 <= 362; i += 12 {
		id := data[i]
		if id == 0 {
			continue
		}
		attr := smartAttr{
			Id:    id,
			Value: data[i+3],
			Worst: data[i+4],
			RawLo: uint32(data[i+5]) | uint32(data[i+6])<<8 | uint32(data[i+7])<<16 | uint32(data[i+8])<<24,
			RawHi: uint16(data[i+9]) | uint16(data[i+10])<<8,
		}
		info.SmartAttrs[id] = attr
	}

	// Temperature: attr 0xBE (194 = Temperature) or 0xC2 (194 decimal)
	// Attr 194 = Temperature_Celsius or HDA_Temperature
	for _, tempId := range []byte{0xC2, 0xBE} {
		if a, ok := info.SmartAttrs[tempId]; ok && a.Value > 0 {
			// Raw low byte is current temperature in Celsius for most drives
			t := float64(a.RawLo & 0xFF)
			if t > 0 && t < 100 {
				info.TempC = t
				info.HasTemp = true
				break
			}
		}
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
