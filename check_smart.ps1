# Test SMART via WMI
Write-Output "=== WMI SMART Test ==="
try {
    $smart = Get-WmiObject -Namespace root\wmi -Class MSStorageDriver_FailurePredictStatus -ErrorAction Stop
    $smart | ForEach-Object { Write-Output "Drive: $($_.InstanceName), PredictFailure: $($_.PredictFailure)" }
} catch {
    Write-Output "MSStorageDriver_FailurePredictStatus error: $_"
}

Write-Output ""
Write-Output "=== WMI SMART Data ==="
try {
    $data = Get-WmiObject -Namespace root\wmi -Class MSStorageDriver_ATAPISmartData -ErrorAction Stop
    $data | ForEach-Object { Write-Output "Instance: $($_.InstanceName)" }
} catch {
    Write-Output "MSStorageDriver_ATAPISmartData error: $_"
}

Write-Output ""
Write-Output "=== Physical Drive Access Test ==="
0..3 | ForEach-Object {
    $path = "\\.\PhysicalDrive$_"
    try {
        $handle = [System.IO.File]::Open($path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::ReadWrite, [System.IO.FileShare]::ReadWrite)
        Write-Output "PhysicalDrive$_`: ACCESSIBLE"
        $handle.Close()
    } catch {
        Write-Output "PhysicalDrive$_`: $($_.Exception.Message)"
    }
}

Write-Output ""
Write-Output "=== NVMe via Storage Query ==="
$drives = Get-PhysicalDisk
$drives | ForEach-Object {
    Write-Output "Disk: $($_.FriendlyName), MediaType: $($_.MediaType), BusType: $($_.BusType), DeviceId: $($_.DeviceId)"
    $temp = Get-StorageReliabilityCounter -PhysicalDisk $_ -ErrorAction SilentlyContinue
    if ($temp) {
        Write-Output "  Temperature: $($temp.Temperature)°C, Wear: $($temp.Wear)%"
    }
}
