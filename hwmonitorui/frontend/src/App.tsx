import { useEffect, useState } from 'react'
import { GetSnapshot } from '../wailsjs/go/main/App'
import {
  LineChart, Line, XAxis, YAxis, CartesianGrid, ResponsiveContainer,
} from 'recharts'
import './style.css'

// ── Types ─────────────────────────────────────────────────────────────────────

interface CoreUsage { core: string; usage: number }
interface CPUData { totalUsage: number; cores: number; freqMHz: number; perCore: CoreUsage[] }
interface MemoryData { totalBytes: number; usedBytes: number; availBytes: number; usagePercent: number; swapTotalBytes: number; swapUsedBytes: number; swapPercent: number }
interface TempEntry { name: string; tempC: number }
interface VoltageEntry { name: string; volts: number }
interface FanEntry { name: string; rpm: number }
interface DiskUsage { mountpoint: string; device: string; totalBytes: number; usedBytes: number; freeBytes: number; usagePercent: number }
interface DiskIO { device: string; readBytes: number; writeBytes: number }
interface NVMeEntry { device: string; lifeRemaining: number; spareAvail: number; hasSpare: boolean; powerOnHours: number; mediaErrors: number; tempC: number; hasTemp: boolean }
interface SATAEntry { device: string; lifeRemaining: number; hasLife: boolean; spareAvail: number; hasSpare: boolean; powerOnHours: number; hasHours: boolean; reallocated: number; hasReallocated: boolean; pending: number; hasPending: boolean; tempC: number; hasTemp: boolean }
interface NetworkEntry { interface: string; sentBytes: number; recvBytes: number }
interface RAMSlot { slot: string; bytes: number; type: string; speedMHz: string }
interface SysInfoData { cpu: string; cores: number; threads: number; motherboard: string; bios: string; ramTotal: number; ramSlots: RAMSlot[] }
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

type HistPoint = Record<string, number>

// ── Constants ─────────────────────────────────────────────────────────────────

const MAX_HIST = 60
const POLL_MS  = 1000

const CORE_COLORS = [
  '#4fc3f7','#ef5350','#66bb6a','#ffca28',
  '#ab47bc','#ff7043','#26c6da','#d4e157',
  '#ec407a','#43a047','#ffa726','#7e57c2',
  '#00acc1','#9ccc65','#0288d1','#29b6f6',
]

const GRID = 'rgba(255,255,255,0.06)'

// ── Helpers ───────────────────────────────────────────────────────────────────

const fmtBytes = (b: number): string => {
  if (b >= 1e12) return (b / 1e12).toFixed(1) + ' TB'
  if (b >= 1e9)  return (b / 1e9 ).toFixed(1) + ' GB'
  if (b >= 1e6)  return (b / 1e6 ).toFixed(1) + ' MB'
  if (b >= 1e3)  return (b / 1e3 ).toFixed(1) + ' KB'
  return b.toFixed(0) + ' B'
}
const fmtBps = (b: number) => fmtBytes(b) + '/s'

const tempColor  = (t: number) => t >= 70 ? '#f87171' : t >= 50 ? '#facc15' : '#39FF8E'
const lifeColor  = (p: number) => p <  10 ? '#f87171' : p <  30 ? '#facc15' : '#39FF8E'
const usageColor = (p: number) => p >= 80 ? '#ef4444' : p >= 60 ? '#eab308' : '#39FF8E'

// ── Primitives ────────────────────────────────────────────────────────────────

const Card = ({ title, children, className = '' }: { title: string; children: React.ReactNode; className?: string }) => (
  <div className={`bg-[#161B22] border border-[#30363D] rounded-lg p-4 ${className}`}>
    <h2 className="text-[#00DCB4] font-semibold text-[10px] uppercase tracking-widest mb-3">{title}</h2>
    {children}
  </div>
)

const Bar = ({ pct, color, thin }: { pct: number; color?: string; thin?: boolean }) => (
  <div className={`w-full bg-[#21262D] rounded-full overflow-hidden ${thin ? 'h-1' : 'h-1.5'}`}>
    <div
      className="h-full rounded-full transition-all duration-500"
      style={{ width: `${Math.min(100, Math.max(0, pct))}%`, backgroundColor: color ?? usageColor(pct) }}
    />
  </div>
)

const Row = ({ label, value, sub, dim }: { label: string; value: string; sub?: string; dim?: boolean }) => (
  <div className="flex justify-between items-baseline py-[3px]">
    <span className={`truncate mr-2 ${dim ? 'text-[#6E7681]' : 'text-[#8B949E]'} text-xs`}>{label}</span>
    <span className="font-mono whitespace-nowrap text-xs">
      {value}{sub && <span className="text-[#6E7681] ml-1">{sub}</span>}
    </span>
  </div>
)

// ── Chart primitives ──────────────────────────────────────────────────────────

function ChartPanel({ title, legend, children }: {
  title: string; legend: React.ReactNode; children: React.ReactNode
}) {
  return (
    <div className="bg-[#161B22] border border-[#30363D] rounded-lg p-3 mb-3">
      <div className="text-[#8B949E] text-[10px] font-medium uppercase tracking-widest mb-2">{title}</div>
      {children}
      <div className="mt-2 flex flex-wrap gap-x-4 gap-y-0.5">{legend}</div>
    </div>
  )
}

function Swatch({ color, label, value }: { color: string; label: string; value: string }) {
  return (
    <div className="flex items-center gap-1.5 text-[11px]">
      <div className="w-4 h-[2px] rounded shrink-0" style={{ backgroundColor: color }} />
      <span className="text-[#6E7681]">{label}</span>
      <span className="font-mono font-semibold" style={{ color }}>{value}</span>
    </div>
  )
}

// ── History charts ────────────────────────────────────────────────────────────

function CpuChart({ history, snap }: { history: HistPoint[]; snap: Snapshot }) {
  const cores = [...(snap.cpu?.perCore ?? [])].sort((a, b) => +a.core - +b.core)
  return (
    <ChartPanel
      title={`CPU History — 60 seconds  ·  ${snap.cpu?.totalUsage.toFixed(1)}% total  ·  ${snap.cpu?.cores} cores  ·  ${snap.cpu?.freqMHz.toFixed(0)} MHz`}
      legend={cores.map((c, i) => (
        <Swatch key={i} color={CORE_COLORS[i % CORE_COLORS.length]}
          label={`Core ${c.core}`} value={`${c.usage.toFixed(0)}%`} />
      ))}
    >
      <ResponsiveContainer width="100%" height={130}>
        <LineChart data={history} margin={{ top: 2, right: 2, bottom: 0, left: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke={GRID} vertical={false} />
          <XAxis dataKey="idx" hide />
          <YAxis domain={[0, 100]} hide width={0} />
          {cores.map((_, i) => (
            <Line key={i} type="monotone" dataKey={`c${i}`}
              stroke={CORE_COLORS[i % CORE_COLORS.length]}
              dot={false} strokeWidth={1.5} isAnimationActive={false} />
          ))}
        </LineChart>
      </ResponsiveContainer>
    </ChartPanel>
  )
}

function MemChart({ history, snap }: { history: HistPoint[]; snap: Snapshot }) {
  const m = snap.memory
  const hasSwap = m?.swapTotalBytes > 0
  // Use dynamic Y ceiling so the line isn't crushed at 3% on a 0-100 scale
  const maxPct = Math.max(...history.map(p => Math.max(p.memPct ?? 0, p.swapPct ?? 0)), 10)
  const yMax   = Math.min(100, Math.ceil(maxPct / 10) * 10 + 10)
  return (
    <ChartPanel
      title="Memory History — 60 seconds"
      legend={<>
        <Swatch color="#e040fb" label="RAM"
          value={`${m?.usagePercent.toFixed(1)}%  ${fmtBytes(m?.usedBytes)} / ${fmtBytes(m?.totalBytes)}`} />
        {hasSwap && <Swatch color="#69f0ae" label="Swap"
          value={`${m?.swapPercent.toFixed(1)}%  ${fmtBytes(m?.swapUsedBytes)} / ${fmtBytes(m?.swapTotalBytes)}`} />}
      </>}
    >
      <ResponsiveContainer width="100%" height={130}>
        <LineChart data={history} margin={{ top: 2, right: 2, bottom: 0, left: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke={GRID} vertical={false} />
          <XAxis dataKey="idx" hide />
          <YAxis domain={[0, yMax]} hide width={0} />
          <Line type="monotone" dataKey="memPct"  stroke="#e040fb" dot={false} strokeWidth={1.5} isAnimationActive={false} />
          {hasSwap && <Line type="monotone" dataKey="swapPct" stroke="#69f0ae" dot={false} strokeWidth={1.5} isAnimationActive={false} />}
        </LineChart>
      </ResponsiveContainer>
    </ChartPanel>
  )
}

function NetChart({ history, snap }: { history: HistPoint[]; snap: Snapshot }) {
  if (!snap.network?.length) return null
  const last   = history[history.length - 1] ?? {}
  const maxBps = Math.max(...history.map(p => Math.max(p.recvBps ?? 0, p.sentBps ?? 0)), 1024)
  return (
    <ChartPanel
      title="Network History — 60 seconds"
      legend={<>
        <Swatch color="#40c4ff" label="Receiving" value={fmtBps(last.recvBps ?? 0)} />
        <Swatch color="#ff6e40" label="Sending"   value={fmtBps(last.sentBps ?? 0)} />
      </>}
    >
      <ResponsiveContainer width="100%" height={130}>
        <LineChart data={history} margin={{ top: 2, right: 2, bottom: 0, left: 0 }}>
          <CartesianGrid strokeDasharray="3 3" stroke={GRID} vertical={false} />
          <XAxis dataKey="idx" hide />
          <YAxis domain={[0, maxBps]} hide width={0} />
          <Line type="monotone" dataKey="recvBps" stroke="#40c4ff" dot={false} strokeWidth={1.5} isAnimationActive={false} />
          <Line type="monotone" dataKey="sentBps" stroke="#ff6e40" dot={false} strokeWidth={1.5} isAnimationActive={false} />
        </LineChart>
      </ResponsiveContainer>
    </ChartPanel>
  )
}

// ── Hardware cards ────────────────────────────────────────────────────────────

const SysInfoCard = ({ s }: { s: SysInfoData }) => (
  <Card title="System Info">
    <Row label="CPU"             value={s.cpu} />
    <Row label="Cores / Threads" value={`${s.cores} / ${s.threads}`} />
    <Row label="Motherboard"     value={s.motherboard} />
    <Row label="BIOS"            value={s.bios} />
    <div className="mt-2 mb-1 text-[10px] text-[#6E7681] uppercase tracking-wider">
      RAM — {fmtBytes(s.ramTotal)} total
    </div>
    {s.ramSlots?.map((sl, i) => (
      <div key={i} className="flex justify-between text-[11px] py-[2px] pl-2">
        <span className="text-[#6E7681]">Slot {sl.slot}</span>
        <span className="font-mono text-[#8B949E]">
          {fmtBytes(sl.bytes)}
          {sl.type     && <span className="ml-1.5">{sl.type}</span>}
          {sl.speedMHz && <span className="text-[#6E7681] ml-1">@ {sl.speedMHz} MHz</span>}
        </span>
      </div>
    ))}
  </Card>
)

const CpuCard = ({ d }: { d: CPUData }) => {
  const cores = [...(d.perCore ?? [])].sort((a, b) => +a.core - +b.core)
  return (
    <Card title="CPU">
      <div className="flex justify-between items-center mb-1">
        <span className="text-[#8B949E] text-xs">Total</span>
        <span className="font-mono font-semibold text-sm">{d.totalUsage.toFixed(1)}%</span>
      </div>
      <Bar pct={d.totalUsage} />
      <div className="text-[#6E7681] text-[11px] mt-1.5 mb-3">{d.cores} cores · {d.freqMHz.toFixed(0)} MHz</div>
      <div className="grid grid-cols-2 gap-x-4 gap-y-1.5">
        {cores.map(c => (
          <div key={c.core}>
            <div className="flex justify-between text-[11px] mb-0.5">
              <span className="text-[#6E7681]">Core {c.core}</span>
              <span className="font-mono text-[#8B949E]">{c.usage.toFixed(0)}%</span>
            </div>
            <Bar pct={c.usage} thin />
          </div>
        ))}
      </div>
    </Card>
  )
}

const MemCard = ({ d }: { d: MemoryData }) => (
  <Card title="Memory">
    <div className="space-y-3">
      <div>
        <div className="flex justify-between items-center mb-1">
          <span className="text-[#8B949E] text-xs">RAM</span>
          <span className="font-mono font-semibold">{d.usagePercent.toFixed(1)}%</span>
        </div>
        <Bar pct={d.usagePercent} />
        <div className="text-[11px] text-[#6E7681] mt-1">
          {fmtBytes(d.usedBytes)} / {fmtBytes(d.totalBytes)} · Free: {fmtBytes(d.availBytes)}
        </div>
      </div>
      {d.swapTotalBytes > 0 && (
        <div>
          <div className="flex justify-between items-center mb-1">
            <span className="text-[#8B949E] text-xs">Swap</span>
            <span className="font-mono font-semibold">{d.swapPercent.toFixed(1)}%</span>
          </div>
          <Bar pct={d.swapPercent} />
          <div className="text-[11px] text-[#6E7681] mt-1">
            {fmtBytes(d.swapUsedBytes)} / {fmtBytes(d.swapTotalBytes)}
          </div>
        </div>
      )}
    </div>
  </Card>
)

const TempsCard = ({ temps }: { temps: TempEntry[] }) => {
  if (!temps?.length) return null
  return (
    <Card title="Temperatures">
      {temps.map((t, i) => (
        <div key={i} className="flex items-center gap-2 py-[3px]">
          <span className="text-[#8B949E] truncate flex-1 text-xs">{t.name}</span>
          <div className="w-12 h-1 bg-[#21262D] rounded-full overflow-hidden shrink-0">
            <div className="h-full rounded-full" style={{ width: `${Math.min(100, t.tempC)}%`, backgroundColor: tempColor(t.tempC) }} />
          </div>
          <span className="font-mono text-xs w-9 text-right shrink-0" style={{ color: tempColor(t.tempC) }}>
            {t.tempC.toFixed(0)}°C
          </span>
        </div>
      ))}
    </Card>
  )
}


const FansCard = ({ fans }: { fans: FanEntry[] }) => {
  if (!fans?.length) return null
  return (
    <Card title="Fans">
      {fans.map((f, i) => (
        <div key={i} className="flex justify-between items-center py-[3px]">
          <span className="text-[#8B949E] text-xs truncate mr-2">{f.name}</span>
          <span className="font-mono text-xs whitespace-nowrap">
            {f.rpm.toFixed(0)} <span className="text-[#6E7681]">RPM</span>
          </span>
        </div>
      ))}
    </Card>
  )
}

const NetworkCard = ({ entries }: { entries: NetworkEntry[] }) => {
  if (!entries?.length) return null
  return (
    <Card title="Network — Cumulative">
      {entries.map((e, i) => (
        <div key={i} className={i > 0 ? 'mt-2 pt-2 border-t border-[#21262D]' : ''}>
          <div className="text-[#6E7681] text-[11px] truncate mb-1">{e.interface}</div>
          <div className="flex gap-5 text-xs font-mono">
            <span>↑ <span className="text-[#ff6e40]">{fmtBytes(e.sentBytes)}</span></span>
            <span>↓ <span className="text-[#40c4ff]">{fmtBytes(e.recvBytes)}</span></span>
          </div>
        </div>
      ))}
    </Card>
  )
}

// ── Storage cards ─────────────────────────────────────────────────────────────

const DiskCard = ({ usage, io }: { usage: DiskUsage[]; io: DiskIO[] }) => {
  const sorted = [...(usage ?? [])].sort((a, b) => a.mountpoint.localeCompare(b.mountpoint))
  return (
    <Card title="Disk">
      {sorted.map(d => (
        <div key={d.mountpoint} className="mb-2.5">
          <div className="flex justify-between text-xs mb-1">
            <span className="font-mono text-[#00DCB4] font-medium">{d.mountpoint}</span>
            <span className="font-mono text-[#8B949E]">
              {d.usagePercent.toFixed(1)}%
              <span className="text-[#6E7681] ml-1.5">{fmtBytes(d.usedBytes)} / {fmtBytes(d.totalBytes)}</span>
            </span>
          </div>
          <Bar pct={d.usagePercent} />
        </div>
      ))}
      {io?.length > 0 && (
        <div className="mt-3 pt-2 border-t border-[#21262D]">
          <div className="text-[10px] text-[#6E7681] uppercase tracking-wider mb-1.5">I/O Totals</div>
          {[...io].sort((a, b) => a.device.localeCompare(b.device)).map(d => (
            <div key={d.device} className="flex justify-between text-[11px] py-[2px]">
              <span className="text-[#6E7681] font-mono">{d.device}</span>
              <span className="font-mono">
                ↑ <span className="text-[#ff6e40]">{fmtBytes(d.writeBytes)}</span>
                <span className="mx-1.5 text-[#30363D]">·</span>
                ↓ <span className="text-[#40c4ff]">{fmtBytes(d.readBytes)}</span>
              </span>
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
        <div key={i} className={i > 0 ? 'mt-4 pt-4 border-t border-[#21262D]' : ''}>
          <div className="text-[#E6EDF3] font-medium text-xs mb-1.5 truncate">{e.device}</div>
          <Bar pct={e.lifeRemaining} color={lifeColor(e.lifeRemaining)} thin />
          <div className="flex flex-wrap gap-x-4 gap-y-0.5 text-[11px] mt-1.5">
            <span>Life <span className="font-mono font-bold" style={{ color: lifeColor(e.lifeRemaining) }}>{e.lifeRemaining.toFixed(0)}%</span></span>
            {e.hasSpare && <span className="text-[#8B949E]">Spare <span className="font-mono">{e.spareAvail.toFixed(0)}%</span></span>}
            <span className="text-[#8B949E]">{e.powerOnHours.toFixed(0)} h</span>
            {e.mediaErrors > 0 && <span className="text-yellow-400">⚠ {e.mediaErrors} errors</span>}
            {e.hasTemp && <span style={{ color: tempColor(e.tempC) }}>{e.tempC.toFixed(0)}°C</span>}
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
        <div key={i} className={i > 0 ? 'mt-4 pt-4 border-t border-[#21262D]' : ''}>
          <div className="text-[#E6EDF3] font-medium text-xs mb-1.5 truncate">{e.device}</div>
          <div className="flex flex-wrap gap-x-4 gap-y-0.5 text-[11px]">
            {e.hasLife && <span>Life <span className="font-mono font-bold" style={{ color: lifeColor(e.lifeRemaining) }}>{e.lifeRemaining.toFixed(0)}%</span></span>}
            {e.hasSpare && <span className="text-[#8B949E]">Spare <span className="font-mono">{e.spareAvail.toFixed(0)}%</span></span>}
            {e.hasHours && <span className="text-[#8B949E]">{e.powerOnHours.toFixed(0)} h</span>}
            {e.hasReallocated && <span className={e.reallocated > 0 ? 'text-yellow-400' : 'text-[#6E7681]'}>
              {e.reallocated > 0 ? `⚠ ${e.reallocated}` : '0'} realloc
            </span>}
            {e.hasPending && <span className={e.pending > 0 ? 'text-red-400' : 'text-[#6E7681]'}>
              {e.pending > 0 ? `⚠ ${e.pending}` : '0'} pending
            </span>}
            {e.hasTemp && <span style={{ color: tempColor(e.tempC) }}>{e.tempC.toFixed(0)}°C</span>}
          </div>
        </div>
      ))}
    </Card>
  )
}

// ── Root ──────────────────────────────────────────────────────────────────────

type Tab = 'resources' | 'hardware' | 'storage'

export default function App() {
  const [snap, setSnap]       = useState<Snapshot | null>(null)
  const [history, setHistory] = useState<HistPoint[]>([])
  const [error, setError]     = useState<string | null>(null)
  const [loading, setLoading] = useState(true)
  const [tab, setTab]         = useState<Tab>('resources')

  useEffect(() => {
    const prevNet: { iface: string; sent: number; recv: number }[] = []
    let prevTime = 0
    let idx = 0

    const fetch = () => {
      GetSnapshot()
        .then((s: Snapshot) => {
          setSnap(s)
          setError(null)
          setLoading(false)

          const now = s.timestamp
          const dt  = prevTime ? (now - prevTime) / 1000 : 0
          prevTime  = now

          let recvBps = 0, sentBps = 0
          if (dt > 0 && prevNet.length && s.network?.length) {
            for (const cur of s.network) {
              const prev = prevNet.find(p => p.iface === cur.interface)
              if (prev) {
                recvBps += Math.max(0, cur.recvBytes - prev.recv) / dt
                sentBps += Math.max(0, cur.sentBytes - prev.sent) / dt
              }
            }
          }
          prevNet.length = 0
          prevNet.push(...(s.network?.map(n => ({ iface: n.interface, sent: n.sentBytes, recv: n.recvBytes })) ?? []))

          const cores = [...(s.cpu?.perCore ?? [])].sort((a, b) => +a.core - +b.core)
          const pt: HistPoint = {
            idx: idx++,
            memPct:  s.memory?.usagePercent ?? 0,
            swapPct: s.memory?.swapPercent  ?? 0,
            recvBps, sentBps,
          }
          cores.forEach((c, i) => { pt[`c${i}`] = c.usage })

          setHistory(prev => {
            const next = [...prev, pt]
            return next.length > MAX_HIST ? next.slice(next.length - MAX_HIST) : next
          })
        })
        .catch((e: unknown) => { setError(String(e)); setLoading(false) })
    }

    fetch()
    const id = setInterval(fetch, POLL_MS)
    return () => clearInterval(id)
  }, [])

  if (loading) return (
    <div className="flex items-center justify-center h-screen text-[#8B949E] text-sm">Collecting data…</div>
  )
  if (error) return (
    <div className="flex items-center justify-center h-screen text-red-400 text-sm px-8 text-center">Error: {error}</div>
  )
  if (!snap) return null

  const ts = new Date(snap.timestamp).toLocaleTimeString()

  const TAB = (t: Tab, label: string) => (
    <button
      onClick={() => setTab(t)}
      className={`px-5 py-2 text-xs font-medium cursor-pointer border-b-2 transition-colors ${
        tab === t
          ? 'border-[#00DCB4] text-[#00DCB4]'
          : 'border-transparent text-[#8B949E] hover:text-[#E6EDF3] hover:border-[#30363D]'
      }`}
    >{label}</button>
  )

  return (
    <div className="flex flex-col h-screen bg-[#0D1117] overflow-hidden">

      {/* Header */}
      <div className="flex items-center justify-between px-5 py-2.5 border-b border-[#21262D] shrink-0">
        <span className="text-[#00DCB4] font-bold text-sm tracking-wide">HWmonitor</span>
        <div className="flex items-center gap-3 text-[11px] text-[#8B949E]">
          {snap.sysinfo?.motherboard && <span>{snap.sysinfo.motherboard}</span>}
          <span className="text-[#21262D]">|</span>
          <span>Updated {ts}</span>
          <div className="w-1.5 h-1.5 rounded-full bg-[#39FF8E] animate-pulse" />
        </div>
      </div>

      {/* Tab bar */}
      <div className="flex border-b border-[#21262D] shrink-0 px-2 bg-[#0D1117]">
        {TAB('resources', 'Resources')}
        {TAB('hardware',  'Hardware')}
        {TAB('storage',   'Storage')}
      </div>

      {/* Content */}
      <div className="flex-1 overflow-y-auto p-4">

        {/* ── Resources ── */}
        {tab === 'resources' && (
          <div className="max-w-6xl mx-auto">
            <CpuChart history={history} snap={snap} />
            <MemChart history={history} snap={snap} />
            <NetChart history={history} snap={snap} />
          </div>
        )}

        {/* ── Hardware ── */}
        {tab === 'hardware' && (
          <div className="grid gap-4" style={{ gridTemplateColumns: 'minmax(0,1fr) minmax(0,1.4fr) minmax(0,1fr)' }}>

            {/* Col 1: SysInfo + Network */}
            <div className="flex flex-col gap-4">
              {snap.sysinfo && <SysInfoCard s={snap.sysinfo} />}
              <NetworkCard entries={snap.network} />
            </div>

            {/* Col 2: CPU (tall) */}
            {snap.cpu && <CpuCard d={snap.cpu} />}

            {/* Col 3: Memory + Fans + Temps */}
            <div className="flex flex-col gap-4">
              {snap.memory  && <MemCard d={snap.memory} />}
              <FansCard fans={snap.fans} />
              <TempsCard temps={snap.temps} />
            </div>

          </div>
        )}

        {/* ── Hardware row 2: Voltages full-width ── */}
        {tab === 'hardware' && snap.voltages?.length > 0 && (
          <div className="mt-4">
            <Card title="Voltages">
              <div className="grid grid-cols-2 md:grid-cols-3 xl:grid-cols-4 gap-x-8">
                {(() => {
                  const cpu = snap.voltages.filter(v => !v.name.startsWith('BMC '))
                  const bmc = snap.voltages.filter(v =>  v.name.startsWith('BMC '))
                  return <>
                    {cpu.length > 0 && (
                      <div>
                        <div className="text-[10px] text-[#6E7681] uppercase tracking-wider mb-1">CPU</div>
                        {cpu.map((v, i) => <Row key={i} label={v.name} value={v.volts.toFixed(3)} sub="V" />)}
                      </div>
                    )}
                    {bmc.map((v, i) => (
                      <Row key={i} label={v.name.replace('BMC ', '')} value={v.volts.toFixed(3)} sub="V" dim />
                    ))}
                  </>
                })()}
              </div>
            </Card>
          </div>
        )}

        {/* ── Storage ── */}
        {tab === 'storage' && (
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-3 gap-4">
            <DiskCard usage={snap.diskUsage} io={snap.diskIO} />
            <NVMeCard entries={snap.nvme} />
            <SATACard entries={snap.sata} />
          </div>
        )}

      </div>
    </div>
  )
}
