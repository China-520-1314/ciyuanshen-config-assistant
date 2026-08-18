export namespace main {

	export class AppInfo {
	    name: string;
	    version: string;
	    updateManifestUrl: string;
	    gatewayUrl: string;

	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.version = source["version"];
	        this.updateManifestUrl = source["updateManifestUrl"];
	        this.gatewayUrl = source["gatewayUrl"];
	    }
	}
	export class BackupFile {
	    clientId: string;
	    originalPath: string;
	    backupPath: string;
	    exists: boolean;

	    static createFrom(source: any = {}) {
	        return new BackupFile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.originalPath = source["originalPath"];
	        this.backupPath = source["backupPath"];
	        this.exists = source["exists"];
	    }
	}
	export class BackupInfo {
	    id: string;
	    // Go type: time
	    createdAt: any;
	    path: string;
	    files: BackupFile[];

	    static createFrom(source: any = {}) {
	        return new BackupInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.path = source["path"];
	        this.files = this.convertValues(source["files"], BackupFile);
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
	export class ClientConnectionResult {
	    id: string;
	    name: string;
	    success: boolean;
	    configured: boolean;
	    status: number;
	    endpoint: string;
	    message: string;
	    // Go type: time
	    checkedAt: any;

	    static createFrom(source: any = {}) {
	        return new ClientConnectionResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.success = source["success"];
	        this.configured = source["configured"];
	        this.status = source["status"];
	        this.endpoint = source["endpoint"];
	        this.message = source["message"];
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
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
	export class ClientStatus {
	    id: string;
	    name: string;
	    installed: boolean;
	    executablePath: string;
	    configPath: string;
	    configExists: boolean;
	    configState: string;
	    version: string;
	    detail: string;

	    static createFrom(source: any = {}) {
	        return new ClientStatus(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.executablePath = source["executablePath"];
	        this.configPath = source["configPath"];
	        this.configExists = source["configExists"];
	        this.configState = source["configState"];
	        this.version = source["version"];
	        this.detail = source["detail"];
	    }
	}
	export class FilePreview {
	    clientId: string;
	    path: string;
	    action: string;

	    static createFrom(source: any = {}) {
	        return new FilePreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.path = source["path"];
	        this.action = source["action"];
	    }
	}
	export class ConfigurationPreview {
	    files: FilePreview[];
	    warnings: string[];
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new ConfigurationPreview(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = this.convertValues(source["files"], FilePreview);
	        this.warnings = source["warnings"];
	        this.error = source["error"];
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
	export class ConfigurationRequest {
	    apiKey: string;
	    targets: string[];
	    models: Record<string, string>;

	    static createFrom(source: any = {}) {
	        return new ConfigurationRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.targets = source["targets"];
	        this.models = source["models"];
	    }
	}
	export class ConfigureResult {
	    success: boolean;
	    backup?: BackupInfo;
	    files: FilePreview[];
	    warnings: string[];
	    error?: string;
	    configured: string[];
	    // Go type: time
	    finishedAt: any;

	    static createFrom(source: any = {}) {
	        return new ConfigureResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.backup = this.convertValues(source["backup"], BackupInfo);
	        this.files = this.convertValues(source["files"], FilePreview);
	        this.warnings = source["warnings"];
	        this.error = source["error"];
	        this.configured = source["configured"];
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
	export class ConnectionCheckReport {
	    results: ClientConnectionResult[];
	    // Go type: time
	    checkedAt: any;

	    static createFrom(source: any = {}) {
	        return new ConnectionCheckReport(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.results = this.convertValues(source["results"], ClientConnectionResult);
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
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
	export class ConnectionCheckRequest {
	    apiKey: string;
	    targets: string[];

	    static createFrom(source: any = {}) {
	        return new ConnectionCheckRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.apiKey = source["apiKey"];
	        this.targets = source["targets"];
	    }
	}
	export class EnvironmentReport {
	    os: string;
	    home: string;
	    // Go type: time
	    scannedAt: any;
	    clients: ClientStatus[];

	    static createFrom(source: any = {}) {
	        return new EnvironmentReport(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.os = source["os"];
	        this.home = source["home"];
	        this.scannedAt = this.convertValues(source["scannedAt"], null);
	        this.clients = this.convertValues(source["clients"], ClientStatus);
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

	export class GroupRatio {
	    name: string;
	    description: string;
	    ratio: number;

	    static createFrom(source: any = {}) {
	        return new GroupRatio(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.ratio = source["ratio"];
	    }
	}
	export class GroupRatioReport {
	    groups: GroupRatio[];
	    endpoint: string;
	    // Go type: time
	    fetchedAt: any;

	    static createFrom(source: any = {}) {
	        return new GroupRatioReport(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.groups = this.convertValues(source["groups"], GroupRatio);
	        this.endpoint = source["endpoint"];
	        this.fetchedAt = this.convertValues(source["fetchedAt"], null);
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
	export class Model {
	    id: string;
	    object?: string;
	    owned_by?: string;

	    static createFrom(source: any = {}) {
	        return new Model(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.object = source["object"];
	        this.owned_by = source["owned_by"];
	    }
	}
	export class ModelResponse {
	    models: Model[];
	    status: number;
	    message?: string;
	    endpoint: string;

	    static createFrom(source: any = {}) {
	        return new ModelResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.models = this.convertValues(source["models"], Model);
	        this.status = source["status"];
	        this.message = source["message"];
	        this.endpoint = source["endpoint"];
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
	export class UpdateInfo {
	    currentVersion: string;
	    latestVersion: string;
	    updateAvailable: boolean;
	    downloadUrl?: string;
	    releaseNotes?: string;
	    publishedAt?: string;
	    sha256?: string;
	    // Go type: time
	    checkedAt: any;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.downloadUrl = source["downloadUrl"];
	        this.releaseNotes = source["releaseNotes"];
	        this.publishedAt = source["publishedAt"];
	        this.sha256 = source["sha256"];
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
	        this.error = source["error"];
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

