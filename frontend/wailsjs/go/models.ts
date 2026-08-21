export namespace connection {
	
	export class Config {
	    protocol: string;
	    host: string;
	    port: number;
	    user: string;
	    password: string;
	    encryption: string;
	    authType: string;
	    sshKeyPath: string;
	    timeoutSec: number;
	    passive: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.protocol = source["protocol"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.encryption = source["encryption"];
	        this.authType = source["authType"];
	        this.sshKeyPath = source["sshKeyPath"];
	        this.timeoutSec = source["timeoutSec"];
	        this.passive = source["passive"];
	    }
	}
	export class ConnInfo {
	    id: string;
	    name: string;
	    host: string;
	    protocol: string;
	    port: number;
	    user: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.host = source["host"];
	        this.protocol = source["protocol"];
	        this.port = source["port"];
	        this.user = source["user"];
	    }
	}
	export class RemoteFileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    size: number;
	    // Go type: time
	    modTime: any;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteFileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.modTime = this.convertValues(source["modTime"], null);
	        this.mode = source["mode"];
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

export namespace fs {
	
	export class FileEntry {
	    name: string;
	    path: string;
	    isDir: boolean;
	    size: number;
	    // Go type: time
	    modTime: any;
	    mode: string;
	
	    static createFrom(source: any = {}) {
	        return new FileEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.isDir = source["isDir"];
	        this.size = source["size"];
	        this.modTime = this.convertValues(source["modTime"], null);
	        this.mode = source["mode"];
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
	
	export class ImportFileInfo {
	    path: string;
	    needsPassphrase: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ImportFileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.needsPassphrase = source["needsPassphrase"];
	    }
	}

}

export namespace settings {
	
	export class Settings {
	    theme: string;
	    language: string;
	    maxConcurrentTransfers: number;
	    defaultLocalDir: string;
	    defaultPort: number;
	    connectionTimeoutSec: number;
	    showHiddenFiles: boolean;
	    passiveMode: boolean;
	    autoReconnect: boolean;
	    confirmOnDelete: boolean;
	    dateFormat: string;
	    maxTransferSpeedKBps: number;
	    accentColor: string;
	    maxConnections: number;
	    connectCardShadow: boolean;
	    windowWidth: number;
	    windowHeight: number;
	    startMaximized: boolean;
	    closeSiteManagerOnClickOutside: boolean;
	    doubleClickNavigateUp: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.theme = source["theme"];
	        this.language = source["language"];
	        this.maxConcurrentTransfers = source["maxConcurrentTransfers"];
	        this.defaultLocalDir = source["defaultLocalDir"];
	        this.defaultPort = source["defaultPort"];
	        this.connectionTimeoutSec = source["connectionTimeoutSec"];
	        this.showHiddenFiles = source["showHiddenFiles"];
	        this.passiveMode = source["passiveMode"];
	        this.autoReconnect = source["autoReconnect"];
	        this.confirmOnDelete = source["confirmOnDelete"];
	        this.dateFormat = source["dateFormat"];
	        this.maxTransferSpeedKBps = source["maxTransferSpeedKBps"];
	        this.accentColor = source["accentColor"];
	        this.maxConnections = source["maxConnections"];
	        this.connectCardShadow = source["connectCardShadow"];
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.startMaximized = source["startMaximized"];
	        this.closeSiteManagerOnClickOutside = source["closeSiteManagerOnClickOutside"];
	        this.doubleClickNavigateUp = source["doubleClickNavigateUp"];
	    }
	}

}

export namespace sites {
	
	export class Site {
	    id: string;
	    name: string;
	    protocol: string;
	    host: string;
	    port: number;
	    encryption: string;
	    authType: string;
	    user: string;
	    password: string;
	    sshKeyPath: string;
	    remoteDir: string;
	    note: string;
	
	    static createFrom(source: any = {}) {
	        return new Site(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.protocol = source["protocol"];
	        this.host = source["host"];
	        this.port = source["port"];
	        this.encryption = source["encryption"];
	        this.authType = source["authType"];
	        this.user = source["user"];
	        this.password = source["password"];
	        this.sshKeyPath = source["sshKeyPath"];
	        this.remoteDir = source["remoteDir"];
	        this.note = source["note"];
	    }
	}

}

export namespace transfer {
	
	export class Job {
	    id: string;
	    direction: string;
	    localPath: string;
	    remotePath: string;
	    remoteHost: string;
	    name: string;
	    size: number;
	    bytesDone: number;
	    status: string;
	    error: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    finishedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.direction = source["direction"];
	        this.localPath = source["localPath"];
	        this.remotePath = source["remotePath"];
	        this.remoteHost = source["remoteHost"];
	        this.name = source["name"];
	        this.size = source["size"];
	        this.bytesDone = source["bytesDone"];
	        this.status = source["status"];
	        this.error = source["error"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.finishedAt = this.convertValues(source["finishedAt"], null);
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

