export namespace invite {
	
	export class Bundle {
	    blob: string;
	    shortCode: string;
	    expiresAt: number;
	    deviceName: string;
	
	    static createFrom(source: any = {}) {
	        return new Bundle(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.blob = source["blob"];
	        this.shortCode = source["shortCode"];
	        this.expiresAt = source["expiresAt"];
	        this.deviceName = source["deviceName"];
	    }
	}

}

export namespace chat {
	
	export class Message {
	    ts: number;
	    peerId: string;
	    peerName: string;
	    dir: string;
	    text: string;
	    read?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ts = source["ts"];
	        this.peerId = source["peerId"];
	        this.peerName = source["peerName"];
	        this.dir = source["dir"];
	        this.text = source["text"];
	        this.read = source["read"];
	    }
	}

}

export namespace netif {
	
	export class NetInterface {
	    name: string;
	    addresses: string[];
	    kind: string;
	    up: boolean;
	    speedMbps: number;
	
	    static createFrom(source: any = {}) {
	        return new NetInterface(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.addresses = source["addresses"];
	        this.kind = source["kind"];
	        this.up = source["up"];
	        this.speedMbps = source["speedMbps"];
	    }
	}

}

export namespace protocol {
	
	export class DeviceInfo {
	    id: string;
	    name: string;
	    host: string;
	    address: string;
	    platform: string;
	    controlPort: number;
	    fingerprint: string;
	    version: number;
	    capabilities?: string[];
	    browser?: string;
	
	    static createFrom(source: any = {}) {
	        return new DeviceInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.address = source["address"];
	        this.platform = source["platform"];
	        this.controlPort = source["controlPort"];
	        this.fingerprint = source["fingerprint"];
	        this.version = source["version"];
	        this.capabilities = source["capabilities"];
	        this.browser = source["browser"];
	    }
	}
	export class SendItem {
	    path: string;
	    relPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SendItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.relPath = source["relPath"];
	    }
	}

}

export namespace staging {
	
	export class Entry {
	    path: string;
	    name: string;
	    kind: string;
	    relPath: string;
	    size: number;
	    fileCount: number;
	    children?: Entry[];
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.name = source["name"];
	        this.kind = source["kind"];
	        this.relPath = source["relPath"];
	        this.size = source["size"];
	        this.fileCount = source["fileCount"];
	        this.children = this.convertValues(source["children"], Entry);
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

export namespace main {
	
	export class ImportInviteResult {
	    device: protocol.DeviceInfo;
	    shortCode: string;
	    expiresAt: number;
	
	    static createFrom(source: any = {}) {
	        return new ImportInviteResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.device = this.convertValues(source["device"], protocol.DeviceInfo);
	        this.shortCode = source["shortCode"];
	        this.expiresAt = source["expiresAt"];
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
