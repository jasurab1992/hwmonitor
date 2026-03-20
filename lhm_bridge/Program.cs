using System;
using System.Collections.Generic;
using System.Text;
using System.Text.Json;
using System.Text.Json.Serialization;
using LibreHardwareMonitor.Hardware;

// lhm_bridge: collects hardware sensors via LibreHardwareMonitor and writes
// a JSON array to stdout. Requires Administrator privileges.

var computer = new Computer
{
    IsCpuEnabled         = true,
    IsMotherboardEnabled = true,
    IsMemoryEnabled      = true,
    IsGpuEnabled         = true,
    IsStorageEnabled     = false,
    IsNetworkEnabled     = false,
};

try
{
    computer.Open();
}
catch (Exception ex)
{
    Console.Error.WriteLine("lhm_bridge: " + ex.Message);
    Environment.Exit(1);
}

computer.Accept(new UpdateVisitor());

var sensors = new List<SensorEntry>();

foreach (var hw in computer.Hardware)
{
    Collect(hw, sensors);
    foreach (var sub in hw.SubHardware)
        Collect(sub, sensors);
}

computer.Close();

Console.OutputEncoding = Encoding.UTF8;
Console.WriteLine(JsonSerializer.Serialize(sensors, AppJsonContext.Default.ListSensorEntry));

static void Collect(IHardware hw, List<SensorEntry> sensors)
{
    hw.Update();
    foreach (var s in hw.Sensors)
    {
        if (s.Value is null) continue;
        sensors.Add(new SensorEntry
        {
            Name       = s.Name,
            Type       = s.SensorType.ToString(),
            Hardware   = hw.Name,
            HwType     = hw.HardwareType.ToString(),
            Identifier = s.Identifier.ToString(),
            Value      = (float)s.Value,
        });
    }
}

class UpdateVisitor : IVisitor
{
    public void VisitComputer(IComputer computer) => computer.Traverse(this);
    public void VisitHardware(IHardware hardware)
    {
        hardware.Update();
        foreach (var sub in hardware.SubHardware)
            sub.Accept(this);
    }
    public void VisitSensor(ISensor sensor) { }
    public void VisitParameter(IParameter parameter) { }
}

class SensorEntry
{
    public string Name       { get; set; } = "";
    public string Type       { get; set; } = "";
    public string Hardware   { get; set; } = "";
    public string HwType     { get; set; } = "";
    public string Identifier { get; set; } = "";
    public float  Value      { get; set; }
}

[JsonSerializable(typeof(List<SensorEntry>))]
[JsonSerializable(typeof(SensorEntry))]
internal partial class AppJsonContext : JsonSerializerContext {}
