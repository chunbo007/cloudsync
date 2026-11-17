export namespace database {
	
	export class Account {
	    id: number;
	    provider: string;
	    name: string;
	    accessKey: string;
	    secretRef: string;
	    bucket: string;
	    bucketCount: number;
	    region: string;
	    cdnDomain: string;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new Account(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider = source["provider"];
	        this.name = source["name"];
	        this.accessKey = source["accessKey"];
	        this.secretRef = source["secretRef"];
	        this.bucket = source["bucket"];
	        this.bucketCount = source["bucketCount"];
	        this.region = source["region"];
	        this.cdnDomain = source["cdnDomain"];
	        this.status = source["status"];
	    }
	}
	export class Bucket {
	    id: number;
	    accountId: number;
	    name: string;
	    region: string;
	    private: boolean;
	    selected: boolean;
	    cdnDomain: string;
	
	    static createFrom(source: any = {}) {
	        return new Bucket(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.name = source["name"];
	        this.region = source["region"];
	        this.private = source["private"];
	        this.selected = source["selected"];
	        this.cdnDomain = source["cdnDomain"];
	    }
	}
	export class Site {
	    id: number;
	    accountId: number;
	    bucketId: number;
	    name: string;
	    remotePrefix: string;
	    localPath: string;
	    versionFile: string;
	    accountName?: string;
	    bucketName?: string;
	    private?: boolean;
	    cdnDomain?: string;
	    buildCommand?: string;
	    buildWorkdir?: string;
	    buildBeforeSync: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Site(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.accountId = source["accountId"];
	        this.bucketId = source["bucketId"];
	        this.name = source["name"];
	        this.remotePrefix = source["remotePrefix"];
	        this.localPath = source["localPath"];
	        this.versionFile = source["versionFile"];
	        this.accountName = source["accountName"];
	        this.bucketName = source["bucketName"];
	        this.private = source["private"];
	        this.cdnDomain = source["cdnDomain"];
	        this.buildCommand = source["buildCommand"];
	        this.buildWorkdir = source["buildWorkdir"];
	        this.buildBeforeSync = source["buildBeforeSync"];
	    }
	}

}

export namespace main {
	
	export class AccountInput {
	    id: number;
	    provider: string;
	    name: string;
	    accessKey: string;
	    secretKey: string;
	
	    static createFrom(source: any = {}) {
	        return new AccountInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.provider = source["provider"];
	        this.name = source["name"];
	        this.accessKey = source["accessKey"];
	        this.secretKey = source["secretKey"];
	    }
	}
	export class BucketSelection {
	    name: string;
	    region: string;
	    private: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BucketSelection(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.region = source["region"];
	        this.private = source["private"];
	    }
	}
	export class DeleteBucketInput {
	    accountId: number;
	    bucket: string;
	    confirm: string;
	
	    static createFrom(source: any = {}) {
	        return new DeleteBucketInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accountId = source["accountId"];
	        this.bucket = source["bucket"];
	        this.confirm = source["confirm"];
	    }
	}
	export class MappingInput {
	    id: number;
	    name: string;
	    localPath: string;
	    remotePrefix: string;
	    buildCommand: string;
	    buildWorkdir: string;
	
	    static createFrom(source: any = {}) {
	        return new MappingInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.localPath = source["localPath"];
	        this.remotePrefix = source["remotePrefix"];
	        this.buildCommand = source["buildCommand"];
	        this.buildWorkdir = source["buildWorkdir"];
	    }
	}
	export class SyncMappingInput {
	    clearRemote: boolean;
	    overwrite: boolean;
	    htmlCacheControl: string;
	    refreshCDN: boolean;
	    cdnDomain: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncMappingInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clearRemote = source["clearRemote"];
	        this.overwrite = source["overwrite"];
	        this.htmlCacheControl = source["htmlCacheControl"];
	        this.refreshCDN = source["refreshCDN"];
	        this.cdnDomain = source["cdnDomain"];
	    }
	}

}

export namespace qiniu {
	
	export class BucketInfo {
	    name: string;
	    region: string;
	    private: boolean;
	
	    static createFrom(source: any = {}) {
	        return new BucketInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.region = source["region"];
	        this.private = source["private"];
	    }
	}
	export class DeleteBucketResult {
	    bucket: string;
	    scanned: number;
	    deleted: number;
	    alreadyAbsent: number;
	
	    static createFrom(source: any = {}) {
	        return new DeleteBucketResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bucket = source["bucket"];
	        this.scanned = source["scanned"];
	        this.deleted = source["deleted"];
	        this.alreadyAbsent = source["alreadyAbsent"];
	    }
	}
	export class RemoteFolder {
	    prefix: string;
	
	    static createFrom(source: any = {}) {
	        return new RemoteFolder(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prefix = source["prefix"];
	    }
	}

}

export namespace syncer {
	
	export class Result {
	    uploaded: number;
	    deleted: number;
	    refreshedUrls: number;
	    localVersion?: string;
	    files: string[];
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.uploaded = source["uploaded"];
	        this.deleted = source["deleted"];
	        this.refreshedUrls = source["refreshedUrls"];
	        this.localVersion = source["localVersion"];
	        this.files = source["files"];
	    }
	}

}

export namespace updater {
	
	export class Manifest {
	    version: string;
	    download: string;
	    sha256: string;
	    notes: string;
	
	    static createFrom(source: any = {}) {
	        return new Manifest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.download = source["download"];
	        this.sha256 = source["sha256"];
	        this.notes = source["notes"];
	    }
	}
	export class Result {
	    available: boolean;
	    current: string;
	    manifest: Manifest;
	
	    static createFrom(source: any = {}) {
	        return new Result(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.available = source["available"];
	        this.current = source["current"];
	        this.manifest = this.convertValues(source["manifest"], Manifest);
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

