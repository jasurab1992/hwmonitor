import { useEffect, useState } from 'react'
import { GetSnapshot } from '../wailsjs/go/main/App'
import './style.css'

// ── Types (mirror app.go) ────────────────────────────────────────────────────

interface CoreUsage { core: string; usage: number }
interface CPUData { totalUsage: number; cores: number; freqMHz: number; perCore: CoreUsage[] }
interface MemoryData { totalBytes: number; usedBytes: number; availBytes: number; usagePercent: number; swapTotalBytes: number; swapUsedBytes: number; swapPercent: number }
interface TempEntry { name: string; tempC: number }
interface VoltageEntry { name: string; volts: number }
interface FanEntry { name: string; rpm: number }
interface DiskUsage { mountpoint: string; device: string; totalBytes: number; usedBytes: number; freeBytes: number; usagePercent: number }
interface DiskIO { device: string; readBytes: number; writeBytes: number }
interface NVMeEntry { device: string; lifeRemaining: number; spareAvail: number; hasSpare: boolean; powerOnHours: number; mediaErrors: number; tempC: number; hasTemp: boolean }
interface SATAEntry { device: string; lifeRemaining: number; hasLife: boolean; powerOnHours: number; hasHours: boolean; reallocated: number; hasReallocated: boolean; pending: number; hasPending: boolean; tempC: number; hasTemp: boolean }
interface NetworkEntry { interface: string; sentBytes: number; recvBytes: number }
interface SysInfoData { cpu: string; cores: number; threads: number; motherboard: string; bios: string; ramTotal: number }
interface Snapshot {
  timestamp: number
  sysinfo: SysInfoData
  cpu: CPUData
  memory: MemoryData
  temps: TempEntry[]
  voltages: VoltageEntry[]
  fans: FanEntry[]
  diskUsage: DiskUsage[]
  diskIO: DiskIO[]
  nvme: NVMeEntry[]
  sata: SATAEntry[]
  network: NetworkEntry[]
}

// ── Helpers ──────────────────────────────────────────────────────────────────

const fmtBytes = (b: number): string => {
  if (b >= 1e12) return (b / 1e12).toFixed(1) + ' TB'
  if (b >= 1e9)  return (b / 1e9).toFixed(1) + ' GB'
  if (b >= 1e6)  return (b / 1e6).toFixed(1) + ' MB'
  if (b >= 1e3)  return (b / 1e3).toFixed(1) + ' KB'
  return b.toFixed(0) + ' B'
}

const tempColor = (t: number) =>
  t >= 70 ? 'text-red-400' : t >= 50 ? 'text-yellow-400' : 'text-green-400'

const tempBarColor = (t: number) =>
  t >= 70 ? 'bg-red-500' : t >= 50 ? 'bg-yellow-400' : 'bg-[#39FF8E]'

const usageBarColor = (p: number) =>
  p >= 80 ? 'bg-red-500' : p >= 60 ? 'bg-yellow-400' : 'bg-[#39FF8E]'

const lifeColor = (p: number) =>
  p < 10 ? 'text-red-400' : p < 30 ? 'text-yellow-400' : 'text-[#39FF8E]'

// ── Reusable components ──────────────────────────────────────────────────────

const Card = ({ title, children }: { title: string; children: React.ReactNode }) => (
  <div className="bg-[#161B22] border border-[#30363D] rounded-lg p-4">
    <h2 className="text-[#00DCB4] font-semibold text-xs uppercase tracking-widest mb-3">
      {title}
    </h2>
    {children}
  </div>
)

const Bar = ({ pct, colorClass }: { pct: number; colorClass?: string }) => (
  <div className="h-1.5 w-full bg-[#30363D] rounded-full overflow-hidden">
    <div
      className={`h-full rounded-full transition-all duration-500 ${colorClass ?? usageBarColor(pct)}`}
      style={{ width: `${Math.min(100, Math.max(0, pct))}%` }}
    />
  </div>
)

const Row = ({ label, value, sub }: { label: string; value: string; sub?: string }) => (
  <div className="flex justify-between items-baseline py-0.5">
    <span className="text-[#8B949E] truncate mr-2">{label}</span>
    <span className="font-mono whitespace-nowrap">
      {value}{sub && <span className="text-[#8B949E] ml-1 text-xs">{sub}</span>}
    </span>
  </div>
)

// ── Section cards ─────────────────────────────────────────────────────────────

const SysInfoCard = ({ s }: { s: SysInfoData }) => (
  <Card title="System Info">
    <Row label="CPU" value={s.cpu} />
    <Row label="Cores / Threads" value={`${s.cores} / ${s.threads}`} />
    <Row label="Motherboard" value={s.motherboard} />
    <Row label="BIOS" value={s.bios} />
    <Row label="RAM" value={fmtBytes(s.ramTotal)} />
  </Card>
)

const CPUCard = ({ d }: { d: CPUData }) => {
  const sorted = [...(d.perCore ?? [])].sort((a, b) => +a.core - +b.core)
  return (
    <Card title="CPU">
      <div className="flex justify-between mb-1">
        <span className="text-[#8B949E]">Total usage</span>
        <span className="font-mono">{d.totalUsage.toFixed(1)}%</span>
      </div>
      <Bar pct={d.totalUsage} />
      <div className="mt-1 text-[#8B949E] text-xs">{d.cores} cores · {d.freqMHz.toFixed(0)} MHz</div>
      {sorted.length > 0 && (
        <div className="mt-3 grid grid-cols-2 gap-x-4 gap-y-1.5">
          {sorted.map(c => (
            <div key={c.core}>
              <div className="flex justify-between text-xs mb-0.5">
                <span className="text-[#8B949E]">Core {c.core}</span>
                <span className="font-mono">{c.usage.toFixed(0)}%</span>
              </div>
              <Bar pct={c.usage} />
            </div>
          ))}
        </div>
      )}
    </Card>
  )
}

const MemoryCard = ({ d }: { d: MemoryData }) => (
  <Card title="Memory">
    <div className="flex justify-between mb-1">
      <span className="text-[#8B949E]">RAM</span>
      <span className="font-mono">{d.usagePercent.toFixed(1)}%</span>
    </div>
    <Bar pct={d.usagePercent} />
    <div className="text-xs text-[#8B949E] mt-1">
      {fmtBytes(d.usedBytes)} / {fmtBytes(d.totalBytes)} · Free: {fmtBytes(d.availBytes)}
    </div>
    {d.swapTotalBytes > 0 && <>
      <div className="flex justify-between mb-1 mt-3">
        <span className="text-[#8B949E]">Swap</span>
        <span className="font-mono">{d.swapPercent.toFixed(1)}%</span>
      </div>
      <Bar pct={d.swapPercent} />
      <div className="text-xs text-[#8B949E] mt-1">
        {fmtBytes(d.swapUsedBytes)} / {fmtBytes(d.swapTotalBytes)}
      </div>
    </>}
  </Card>
)

const TempsCard = ({ temps }: { temps: TempEntry[] }) => {
  if (!temps?.length) return null
  return (
    <Card title="Temperatures">
      {temps.map((t, i) => (
        <div key={i} className="flex items-center justify-between py-0.5 gap-2">
          <span className="text-[#8B949E] truncate flex-1 text-xs">{t.name}</span>
          <div className="flex items-center gap-2 shrink-0">
            <div className="w-14 h-1.5 bg-[#30363D] rounded-full overflow-hidden">
              <div className={`h-full rounded-full ${tempBarColor(t.tempC)}`}
                style={{ width: `${Math.min(100, t.tempC)}%` }} />
            </div>
            <span className={`font-mono w-10 text-right text-xs ${tempColor(t.tempC)}`}>
              {t.tempC.toFixed(0)}°C
            </span>
          </div>
        </div>
      ))}
    </Card>
  )
}

const VoltagesCard = ({ voltages }: { voltages: VoltageEntry[] }) => {
  if (!voltages?.length) return null
  return (
    <Card title="Voltages">
      {voltages.map((v, i) => (
        <Row key={i} label={v.name} value={v.volts.toFixed(3)} sub="V" />
      ))}
    </Card>
  )
}

const FansCard = ({ fans }: { fans: FanEntry[] }) => {
  if (!fans?.length) return null
  return (
    <Card title="Fans">
      {fans.map((f, i) => (
        <Row key={i} label={f.name} value={f.rpm.toFixed(0)} sub="RPM" />
      ))}
    </Card>
  )
}

const DiskCard = ({ usage, io }: { usage: DiskUsage[]; io: DiskIO[] }) => {
  const sorted = [...(usage ?? [])].sort((a, b) => a.mountpoint.localeCompare(b.mountpoint))
  return (
    <Card title="Disk">
      {sorted.map(d => (
        <div key={d.mountpoint} className="mb-2">
          <div className="flex justify-between mb-0.5 text-xs">
            <span className="font-mono text-[#00DCB4]">{d.mountpoint}</span>
            <span className="font-mono">{d.usagePercent.toFixed(1)}%
              <span className="text-[#8B949E] ml-1">{fmtBytes(d.usedBytes)} / {fmtBytes(d.totalBytes)}</span>
            </span>
          </div>
          <Bar pct={d.usagePercent} />
        </div>
      ))}
      {io?.length > 0 && (
        <div className="mt-3 border-t border-[#30363D] pt-2">
          <div className="text-[#8B949E] text-xs mb-1">I/O Totals</div>
          {[...io].sort((a, b) => a.device.localeCompare(b.device)).map(d => (
            <div key={d.device} className="flex justify-between text-xs py-0.5">
              <span className="text-[#8B949E] font-mono">{d.device}</span>
              <span>↑ {fmtBytes(d.writeBytes)} · ↓ {fmtBytes(d.readBytes)}</span>
            </div>
          ))}
        </div>
      )}
    </Card>
  )
}

const NVMeCard = ({ entries }: { entries: NVMeEntry[] }) => {
  if (!entries?.length) return null
  return (
    <Card title="NVMe SMART">
      {entries.map((e, i) => (
        <div key={i} className={i > 0 ? 'mt-3 pt-3 border-t border-[#30363D]' : ''}>
          <div className="text-[#E6EDF3] font-medium mb-1 truncate text-xs">{e.device}</div>
          <Bar pct={e.lifeRemaining}
            colorClass={e.lifeRemaining < 10 ? 'bg-red-500' : e.lifeRemaining < 30 ? 'bg-yellow-400' : 'bg-[#39FF8E]'} />
          <div className="flex gap-3 text-xs mt-1 flex-wrap">
            <span>Life <span className={`font-mono font-bold ${lifeColor(e.lifeRemaining)}`}>{e.lifeRemaining.toFixed(0)}%</span></span>
            {e.hasSpare && <span className="text-[#8B949E]">Spare <span className="font-mono">{e.spareAvail.toFixed(0)}%</span></span>}
            <span className="text-[#8B949E]">{e.powerOnHours.toFixed(0)} h</span>
            {e.mediaErrors > 0 && <span className="text-yellow-400">⚠ {e.mediaErrors} errors</span>}
            {e.hasTemp && <span className={tempColor(e.tempC)}>{e.tempC.toFixed(0)}°C</span>}
          </div>
        </div>
      ))}
    </Card>
  )
}

const SATACard = ({ entries }: { entries: SATAEntry[] }) => {
  if (!entries?.length) return null
  return (
    <Card title="SATA SMART">
      {entries.map((e, i) => (
        <div key={i} className={i > 0 ? 'mt-3 pt-3 border-t border-[#30363D]' : ''}>
          <div className="text-[#E6EDF3] font-medium mb-1 truncate text-xs">{e.device}</div>
          <div className="flex gap-3 text-xs flex-wrap">
            {e.hasLife && <span>Life <span className={`font-mono font-bold ${lifeColor(e.lifeRemaining)}`}>{e.lifeRemaining.toFixed(0)}%</span></span>}
            {e.hasHours && <span className="text-[#8B949E]">{e.powerOnHours.toFixed(0)} h</span>}
            {e.hasReallocated && e.reallocated > 0 && <span className="text-yellow-400">⚠ {e.reallocated} realloc</span>}
            {e.hasPending && e.pending > 0 && <span className="text-red-400">⚠ {e.pending} pending</span>}
            {e.hasTemp && <span className={tempColor(e.tempC)}>{e.tempC.toFixed(0)}°C</span>}
          </div>
        </div>
      ))}
    </Card>
  )
}

const NetworkCard = ({ entries }: { entries: NetworkEntry[] }) => {
  if (!entries?.length) return null
  return (
    <Card title="Network">
      {entries.map((e, i) => (
        <div key={i} className="py-1">
          <div className="text-[#8B949E] text-xs truncate mb-0.5">{e.interface}</div>
          <div className="flex gap-4 font-mono text-xs">
            <span>↑ {fmtBytes(e.sentBytes)}</span>
            <span>↓ {fmtBytes(e.recvBytes)}</span>
          </div>
        </div>
      ))}
    </Card>
  )
}

// ── Root ─────────────────────────────────────────────────────────────────────

export default function App() {
  const [snap, setSnap] = useState<Snapshot | null>(null)
  const [error, setError] = useState<string | null>(null)
  const [loading, setLoading] = useState(true)

  const refresh = () => {
    GetSnapshot()
      .then((s: Snapshot) => { setSnap(s); setError(null); setLoading(false) })
      .catch((e: unknown) => { setError(String(e)); setLoading(false) })
  }

  useEffect(() => {
    refresh()
    const id = setInterval(refresh, 3000)
    return () => clearInterval(id)
  }, [])

  if (loading) return (
    <div className="flex items-center justify-center h-screen text-[#8B949E]">
      Collecting data…
    </div>
  )
  if (error) return (
    <div className="flex items-center justify-center h-screen text-red-400">Error: {error}</div>
  )
  if (!snap) return null

  const ts = new Date(snap.timestamp).toLocaleTimeString()

  return (
    <div className="min-h-screen bg-[#0D1117] p-4">
      {/* Header */}
      <div className="flex items-center justify-between mb-4 pb-3 border-b border-[#30363D]">
        <h1 className="text-[#00DCB4] font-bold text-base tracking-wide">
          HWmonitor
        </h1>
        <div className="flex items-center gap-3 text-xs">
          {snap.sysinfo?.motherboard && (
            <span className="text-[#8B949E]">{snap.sysinfo.motherboard}</span>
          )}
          <span className="text-[#30363D]">·</span>
          <span className="text-[#8B949E]">Updated {ts}</span>
          <div className="w-2 h-2 rounded-full bg-[#39FF8E] animate-pulse" />
        </div>
      </div>

      {/* Grid */}
      <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
        {snap.sysinfo && <SysInfoCard s={snap.sysinfo} />}
        {snap.cpu     && <CPUCard d={snap.cpu} />}
        {snap.memory  && <MemoryCard d={snap.memory} />}
        <TempsCard temps={snap.temps} />
        <VoltagesCard voltages={snap.voltages} />
        <FansCard fans={snap.fans} />
        <DiskCard usage={snap.diskUsage} io={snap.diskIO} />
        <NVMeCard entries={snap.nvme} />
        <SATACard entries={snap.sata} />
        <NetworkCard entries={snap.network} />
      </div>
    </div>
  )
}
