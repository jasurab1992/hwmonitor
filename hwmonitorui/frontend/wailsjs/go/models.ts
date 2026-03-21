export namespace main {
	
	export class CoreUsage {
	    core: string;
	    usage: number;
	
	    static createFrom(source: any = {}) {
	        return new CoreUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.core = source["core"];
	        this.usage = source["usage"];
	    }
	}
	export class CPUData {
	    totalUsage: number;
	    cores: number;
	    freqMHz: number;
	    perCore: CoreUsage[];
	
	    static createFrom(source: any = {}) {
	        return new CPUData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalUsage = source["totalUsage"];
	        this.cores = source["cores"];
	        this.freqMHz = source["freqMHz"];
	        this.perCore = this.convertValues(source["perCore"], CoreUsage);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class DiskIO {
	    device: string;
	    readBytes: number;
	    writeBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskIO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.readBytes = source["readBytes"];
	        this.writeBytes = source["writeBytes"];
	    }
	}
	export class DiskUsage {
	    mountpoint: string;
	    device: string;
	    totalBytes: number;
	    usedBytes: number;
	    freeBytes: number;
	    usagePercent: number;
	
	    static createFrom(source: any = {}) {
	        return new DiskUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mountpoint = source["mountpoint"];
	        this.device = source["device"];
	        this.totalBytes = source["totalBytes"];
	        this.usedBytes = source["usedBytes"];
	        this.freeBytes = source["freeBytes"];
	        this.usagePercent = source["usagePercent"];
	    }
	}
	export class FanEntry {
	    name: string;
	    rpm: number;
	
	    static createFrom(source: any = {}) {
	        return new FanEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.rpm = source["rpm"];
	    }
	}
	export class MemoryData {
	    totalBytes: number;
	    usedBytes: number;
	    availBytes: number;
	    usagePercent: number;
	    swapTotalBytes: number;
	    swapUsedBytes: number;
	    swapPercent: number;
	
	    static createFrom(source: any = {}) {
	        return new MemoryData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalBytes = source["totalBytes"];
	        this.usedBytes = source["usedBytes"];
	        this.availBytes = source["availBytes"];
	        this.usagePercent = source["usagePercent"];
	        this.swapTotalBytes = source["swapTotalBytes"];
	        this.swapUsedBytes = source["swapUsedBytes"];
	        this.swapPercent = source["swapPercent"];
	    }
	}
	export class NVMeEntry {
	    device: string;
	    lifeRemaining: number;
	    spareAvail: number;
	    hasSpare: boolean;
	    powerOnHours: number;
	    mediaErrors: number;
	    tempC: number;
	    hasTemp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new NVMeEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.lifeRemaining = source["lifeRemaining"];
	        this.spareAvail = source["spareAvail"];
	        this.hasSpare = source["hasSpare"];
	        this.powerOnHours = source["powerOnHours"];
	        this.mediaErrors = source["mediaErrors"];
	        this.tempC = source["tempC"];
	        this.hasTemp = source["hasTemp"];
	    }
	}
	export class NetworkEntry {
	    interface: string;
	    sentBytes: number;
	    recvBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new NetworkEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.interface = source["interface"];
	        this.sentBytes = source["sentBytes"];
	        this.recvBytes = source["recvBytes"];
	    }
	}
	export class SATAEntry {
	    device: string;
	    lifeRemaining: number;
	    hasLife: boolean;
	    powerOnHours: number;
	    hasHours: boolean;
	    reallocated: number;
	    hasReallocated: boolean;
	    pending: number;
	    hasPending: boolean;
	    tempC: number;
	    hasTemp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SATAEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = source["device"];
	        this.lifeRemaining = source["lifeRemaining"];
	        this.hasLife = source["hasLife"];
	        this.powerOnHours = source["powerOnHours"];
	        this.hasHours = source["hasHours"];
	        this.reallocated = source["reallocated"];
	        this.hasReallocated = source["hasReallocated"];
	        this.pending = source["pending"];
	        this.hasPending = source["hasPending"];
	        this.tempC = source["tempC"];
	        this.hasTemp = source["hasTemp"];
	    }
	}
	export class VoltageEntry {
	    name: string;
	    volts: number;
	
	    static createFrom(source: any = {}) {
	        return new VoltageEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.volts = source["volts"];
	    }
	}
	export class TempEntry {
	    name: string;
	    tempC: number;
	
	    static createFrom(source: any = {}) {
	        return new TempEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.tempC = source["tempC"];
	    }
	}
	export class SysInfoData {
	    cpu: string;
	    cores: number;
	    threads: number;
	    motherboard: string;
	    bios: string;
	    ramTotal: number;
	
	    static createFrom(source: any = {}) {
	        return new SysInfoData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cpu = source["cpu"];
	        this.cores = source["cores"];
	        this.threads = source["threads"];
	        this.motherboard = source["motherboard"];
	        this.bios = source["bios"];
	        this.ramTotal = source["ramTotal"];
	    }
	}
	export class Snapshot {
	    timestamp: number;
	    sysinfo: SysInfoData;
	    cpu: CPUData;
	    memory: MemoryData;
	    temps: TempEntry[];
	    voltages: VoltageEntry[];
	    fans: FanEntry[];
	    diskUsage: DiskUsage[];
	    diskIO: DiskIO[];
	    nvme: NVMeEntry[];
	    sata: SATAEntry[];
	    network: NetworkEntry[];
	
	    static createFrom(source: any = {}) {
	        return new Snapshot(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.timestamp = source["timestamp"];
	        this.sysinfo = this.convertValues(source["sysinfo"], SysInfoData);
	        this.cpu = this.convertValues(source["cpu"], CPUData);
	        this.memory = this.convertValues(source["memory"], MemoryData);
	        this.temps = this.convertValues(source["temps"], TempEntry);
	        this.voltages = this.convertValues(source["voltages"], VoltageEntry);
	        this.fans = this.convertValues(source["fans"], FanEntry);
	        this.diskUsage = this.convertValues(source["diskUsage"], DiskUsage);
	        this.diskIO = this.convertValues(source["diskIO"], DiskIO);
	        this.nvme = this.convertValues(source["nvme"], NVMeEntry);
	        this.sata = this.convertValues(source["sata"], SATAEntry);
	        this.network = this.convertValues(source["network"], NetworkEntry);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	

}

