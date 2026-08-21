export namespace main {

	export class AccountLoginRequest {
	    username: string;
	    password: string;

	    static createFrom(source: any = {}) {
	        return new AccountLoginRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class AccountLoginResult {
	    signedIn: boolean;
	    requiresTwoFactor: boolean;
	    flowToken: string;
	    username: string;
	    // Go type: time
	    expiresAt: any;

	    static createFrom(source: any = {}) {
	        return new AccountLoginResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.signedIn = source["signedIn"];
	        this.requiresTwoFactor = source["requiresTwoFactor"];
	        this.flowToken = source["flowToken"];
	        this.username = source["username"];
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
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
	export class SavedAccountLogin {
	    username: string;
	    password: string;

	    static createFrom(source: any = {}) {
	        return new SavedAccountLogin(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.username = source["username"];
	        this.password = source["password"];
	    }
	}
	export class AccountState {
	    signedIn: boolean;
	    username: string;
	    balance: string;
	    quota: number;
	    // Go type: time
	    balanceUpdatedAt: any;
	    // Go type: time
	    expiresAt: any;

	    static createFrom(source: any = {}) {
	        return new AccountState(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.signedIn = source["signedIn"];
	        this.username = source["username"];
	        this.balance = source["balance"];
	        this.quota = source["quota"];
	        this.balanceUpdatedAt = this.convertValues(source["balanceUpdatedAt"], null);
	        this.expiresAt = this.convertValues(source["expiresAt"], null);
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
	export class AccountTwoFactorRequest {
	    flowToken: string;
	    code: string;

	    static createFrom(source: any = {}) {
	        return new AccountTwoFactorRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.flowToken = source["flowToken"];
	        this.code = source["code"];
	    }
	}
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
	export class ClientConfigurationFile {
	    path: string;
	    exists: boolean;
	    content: string;

	    static createFrom(source: any = {}) {
	        return new ClientConfigurationFile(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.exists = source["exists"];
	        this.content = source["content"];
	    }
	}
	export class ClientConfigurationView {
	    clientId: string;
	    clientName: string;
	    files: ClientConfigurationFile[];
	    secretsRedacted: boolean;

	    static createFrom(source: any = {}) {
	        return new ClientConfigurationView(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.clientName = source["clientName"];
	        this.files = this.convertValues(source["files"], ClientConfigurationFile);
	        this.secretsRedacted = source["secretsRedacted"];
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
	    supported: boolean;
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
	        this.supported = source["supported"];
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
	    targets: string[];

	    static createFrom(source: any = {}) {
	        return new ConnectionCheckRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
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
	export class ExistingToolConfigurationRequest {
	    clientId: string;
	    model: string;

	    static createFrom(source: any = {}) {
	        return new ExistingToolConfigurationRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.model = source["model"];
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
	export class ProvisionedToolConfigurationRequest {
	    provisionId: string;
	    clientId: string;
	    model: string;

	    static createFrom(source: any = {}) {
	        return new ProvisionedToolConfigurationRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provisionId = source["provisionId"];
	        this.clientId = source["clientId"];
	        this.model = source["model"];
	    }
	}
	export class ToolConfigurationRequest {
	    clientId: string;
	    apiKey: string;
	    model: string;

	    static createFrom(source: any = {}) {
	        return new ToolConfigurationRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.apiKey = source["apiKey"];
	        this.model = source["model"];
	    }
	}
	export class ToolGroupOption {
	    name: string;
	    description: string;
	    ratio: string;
	    models: Model[];

	    static createFrom(source: any = {}) {
	        return new ToolGroupOption(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.ratio = source["ratio"];
	        this.models = this.convertValues(source["models"], Model);
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
	export class ToolKeyRequest {
	    clientId: string;
	    group: string;

	    static createFrom(source: any = {}) {
	        return new ToolKeyRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.group = source["group"];
	    }
	}
	export class ToolKeyResult {
	    provisionId: string;
	    clientId: string;
	    group: string;
	    name: string;
	    existing: boolean;
	    models: Model[];
	    status: number;
	    endpoint: string;

	    static createFrom(source: any = {}) {
	        return new ToolKeyResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provisionId = source["provisionId"];
	        this.clientId = source["clientId"];
	        this.group = source["group"];
	        this.name = source["name"];
	        this.existing = source["existing"];
	        this.models = this.convertValues(source["models"], Model);
	        this.status = source["status"];
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
	export class ToolKeyValidationRequest {
	    clientId: string;
	    apiKey: string;

	    static createFrom(source: any = {}) {
	        return new ToolKeyValidationRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.apiKey = source["apiKey"];
	    }
	}
	export class ToolKeyValidationResult {
	    clientId: string;
	    models: Model[];
	    selectedModel?: string;
	    status: number;
	    endpoint: string;

	    static createFrom(source: any = {}) {
	        return new ToolKeyValidationResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.models = this.convertValues(source["models"], Model);
	        this.selectedModel = source["selectedModel"];
	        this.status = source["status"];
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
	export class ToolLifecycleInfo {
	    clientId: string;
	    name: string;
	    installed: boolean;
	    currentVersion?: string;
	    latestVersion?: string;
	    updateAvailable: boolean;
	    canInstall: boolean;
	    canUpdate: boolean;
	    downloadUrl?: string;
	    installMethod?: string;
	    // Go type: time
	    checkedAt: any;
	    message?: string;
	    error?: string;

	    static createFrom(source: any = {}) {
	        return new ToolLifecycleInfo(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.updateAvailable = source["updateAvailable"];
	        this.canInstall = source["canInstall"];
	        this.canUpdate = source["canUpdate"];
	        this.downloadUrl = source["downloadUrl"];
	        this.installMethod = source["installMethod"];
	        this.checkedAt = this.convertValues(source["checkedAt"], null);
	        this.message = source["message"];
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
	export class ToolLifecycleRequest {
	    clientId: string;
	    action: string;

	    static createFrom(source: any = {}) {
	        return new ToolLifecycleRequest(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.action = source["action"];
	    }
	}
	export class ToolLifecycleResult {
	    success: boolean;
	    manual: boolean;
	    downloadUrl?: string;
	    message?: string;
	    error?: string;
	    info: ToolLifecycleInfo;

	    static createFrom(source: any = {}) {
	        return new ToolLifecycleResult(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.success = source["success"];
	        this.manual = source["manual"];
	        this.downloadUrl = source["downloadUrl"];
	        this.message = source["message"];
	        this.error = source["error"];
	        this.info = this.convertValues(source["info"], ToolLifecycleInfo);
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
	export class ToolOptionsResponse {
	    clientId: string;
	    groups: ToolGroupOption[];
	    existingKeys: ToolKeyResult[];

	    static createFrom(source: any = {}) {
	        return new ToolOptionsResponse(source);
	    }

	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.clientId = source["clientId"];
	        this.groups = this.convertValues(source["groups"], ToolGroupOption);
	        this.existingKeys = this.convertValues(source["existingKeys"], ToolKeyResult);
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
