for ($i = 0; $i -le 5; $i++) {
    $disk = Get-Disk -Number $i -ErrorAction SilentlyContinue
    if ($disk) {
        Write-Output "PhysicalDrive$i : $($disk.FriendlyName) [$($disk.BusType)]"
    } else {
        Write-Output "PhysicalDrive$i : not found"
    }
}

Write-Output ""
Write-Output "All physical disks:"
Get-PhysicalDisk | Select-Object DeviceId, FriendlyName, MediaType, BusType, Size | Format-Table -AutoSize
