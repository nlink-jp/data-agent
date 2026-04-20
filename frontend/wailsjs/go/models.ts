export namespace casemgr {
	
	export class CaseInfo {
	    id: string;
	    name: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new CaseInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
	        this.status = source["status"];
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

export namespace config {
	
	export class AnalysisConfig {
	    ContextLimit: number;
	    OverlapRatio: number;
	    MaxFindings: number;
	    MaxRecordsPerWindow: number;
	
	    static createFrom(source: any = {}) {
	        return new AnalysisConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ContextLimit = source["ContextLimit"];
	        this.OverlapRatio = source["OverlapRatio"];
	        this.MaxFindings = source["MaxFindings"];
	        this.MaxRecordsPerWindow = source["MaxRecordsPerWindow"];
	    }
	}
	export class WindowConfig {
	    X: number;
	    Y: number;
	    Width: number;
	    Height: number;
	
	    static createFrom(source: any = {}) {
	        return new WindowConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.X = source["X"];
	        this.Y = source["Y"];
	        this.Width = source["Width"];
	        this.Height = source["Height"];
	    }
	}
	export class TuningConfig {
	    CJKTokenRatio: number;
	    ASCIITokenRatio: number;
	    CharsPerToken: number;
	
	    static createFrom(source: any = {}) {
	        return new TuningConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.CJKTokenRatio = source["CJKTokenRatio"];
	        this.ASCIITokenRatio = source["ASCIITokenRatio"];
	        this.CharsPerToken = source["CharsPerToken"];
	    }
	}
	export class ContainerConfig {
	    Runtime: string;
	    Image: string;
	
	    static createFrom(source: any = {}) {
	        return new ContainerConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Runtime = source["Runtime"];
	        this.Image = source["Image"];
	    }
	}
	export class LocalLLMConfig {
	    Endpoint: string;
	    Model: string;
	    APIKey: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalLLMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Endpoint = source["Endpoint"];
	        this.Model = source["Model"];
	        this.APIKey = source["APIKey"];
	    }
	}
	export class VertexAIConfig {
	    Project: string;
	    Region: string;
	    Model: string;
	
	    static createFrom(source: any = {}) {
	        return new VertexAIConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Project = source["Project"];
	        this.Region = source["Region"];
	        this.Model = source["Model"];
	    }
	}
	export class LLMConfig {
	    Backend: string;
	
	    static createFrom(source: any = {}) {
	        return new LLMConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Backend = source["Backend"];
	    }
	}
	export class Config {
	    LLM: LLMConfig;
	    VertexAI: VertexAIConfig;
	    LocalLLM: LocalLLMConfig;
	    Analysis: AnalysisConfig;
	    Container: ContainerConfig;
	    Tuning: TuningConfig;
	    Window: WindowConfig;
	
	    static createFrom(source: any = {}) {
	        return new Config(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.LLM = this.convertValues(source["LLM"], LLMConfig);
	        this.VertexAI = this.convertValues(source["VertexAI"], VertexAIConfig);
	        this.LocalLLM = this.convertValues(source["LocalLLM"], LocalLLMConfig);
	        this.Analysis = this.convertValues(source["Analysis"], AnalysisConfig);
	        this.Container = this.convertValues(source["Container"], ContainerConfig);
	        this.Tuning = this.convertValues(source["Tuning"], TuningConfig);
	        this.Window = this.convertValues(source["Window"], WindowConfig);
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

export namespace dbengine {
	
	export class ColumnMeta {
	    name: string;
	    type: string;
	    nullable: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ColumnMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.nullable = source["nullable"];
	    }
	}
	export class QueryResult {
	    sql: string;
	    columns: string[];
	    rows: any[];
	    row_count: number;
	    duration: number;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new QueryResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sql = source["sql"];
	        this.columns = source["columns"];
	        this.rows = source["rows"];
	        this.row_count = source["row_count"];
	        this.duration = source["duration"];
	        this.error = source["error"];
	    }
	}
	export class TableMeta {
	    name: string;
	    columns: ColumnMeta[];
	    row_count: number;
	    sample_data?: any[];
	    // Go type: time
	    imported_at: any;
	    source_file: string;
	
	    static createFrom(source: any = {}) {
	        return new TableMeta(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.columns = this.convertValues(source["columns"], ColumnMeta);
	        this.row_count = source["row_count"];
	        this.sample_data = source["sample_data"];
	        this.imported_at = this.convertValues(source["imported_at"], null);
	        this.source_file = source["source_file"];
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

export namespace job {
	
	export class Job {
	    id: string;
	    case_id: string;
	    type: string;
	    status: string;
	    progress: number;
	    error?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Job(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.case_id = source["case_id"];
	        this.type = source["type"];
	        this.status = source["status"];
	        this.progress = source["progress"];
	        this.error = source["error"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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

export namespace report {
	
	export class Report {
	    id: string;
	    case_id: string;
	    session_id: string;
	    title: string;
	    content: string;
	    // Go type: time
	    created_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Report(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.case_id = source["case_id"];
	        this.session_id = source["session_id"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.created_at = this.convertValues(source["created_at"], null);
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

export namespace session {
	
	export class ChatMessage {
	    role: string;
	    content: string;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new ChatMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
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
	export class StepResult {
	    summary: string;
	    data?: string;
	
	    static createFrom(source: any = {}) {
	        return new StepResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.summary = source["summary"];
	        this.data = source["data"];
	    }
	}
	export class ExecEntry {
	    step_id: string;
	    type: string;
	    sql?: string;
	    result?: StepResult;
	    error?: string;
	    decision?: string;
	    duration: number;
	    // Go type: time
	    timestamp: any;
	    plan_version: number;
	
	    static createFrom(source: any = {}) {
	        return new ExecEntry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.step_id = source["step_id"];
	        this.type = source["type"];
	        this.sql = source["sql"];
	        this.result = this.convertValues(source["result"], StepResult);
	        this.error = source["error"];
	        this.decision = source["decision"];
	        this.duration = source["duration"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
	        this.plan_version = source["plan_version"];
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
	export class Finding {
	    id: string;
	    description: string;
	    severity: string;
	    step_id: string;
	    data?: string;
	
	    static createFrom(source: any = {}) {
	        return new Finding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.description = source["description"];
	        this.severity = source["severity"];
	        this.step_id = source["step_id"];
	        this.data = source["data"];
	    }
	}
	export class StepError {
	    message: string;
	    severity: string;
	
	    static createFrom(source: any = {}) {
	        return new StepError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.message = source["message"];
	        this.severity = source["severity"];
	    }
	}
	export class Step {
	    id: string;
	    type: string;
	    description: string;
	    sql?: string;
	    depends_on?: string[];
	    status: string;
	    result?: StepResult;
	    error?: StepError;
	    retry_count: number;
	
	    static createFrom(source: any = {}) {
	        return new Step(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.description = source["description"];
	        this.sql = source["sql"];
	        this.depends_on = source["depends_on"];
	        this.status = source["status"];
	        this.result = this.convertValues(source["result"], StepResult);
	        this.error = this.convertValues(source["error"], StepError);
	        this.retry_count = source["retry_count"];
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
	export class Perspective {
	    id: string;
	    description: string;
	    steps: Step[];
	    status: string;
	
	    static createFrom(source: any = {}) {
	        return new Perspective(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.description = source["description"];
	        this.steps = this.convertValues(source["steps"], Step);
	        this.status = source["status"];
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
	export class PlanRevision {
	    version: number;
	    reason: string;
	    changes: string;
	    // Go type: time
	    timestamp: any;
	
	    static createFrom(source: any = {}) {
	        return new PlanRevision(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.reason = source["reason"];
	        this.changes = source["changes"];
	        this.timestamp = this.convertValues(source["timestamp"], null);
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
	export class Plan {
	    objective: string;
	    perspectives: Perspective[];
	    version: number;
	    history?: PlanRevision[];
	
	    static createFrom(source: any = {}) {
	        return new Plan(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.objective = source["objective"];
	        this.perspectives = this.convertValues(source["perspectives"], Perspective);
	        this.version = source["version"];
	        this.history = this.convertValues(source["history"], PlanRevision);
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
	
	export class Session {
	    id: string;
	    case_id: string;
	    phase: string;
	    plan?: Plan;
	    chat: ChatMessage[];
	    exec_log: ExecEntry[];
	    findings: Finding[];
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    updated_at: any;
	
	    static createFrom(source: any = {}) {
	        return new Session(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.case_id = source["case_id"];
	        this.phase = source["phase"];
	        this.plan = this.convertValues(source["plan"], Plan);
	        this.chat = this.convertValues(source["chat"], ChatMessage);
	        this.exec_log = this.convertValues(source["exec_log"], ExecEntry);
	        this.findings = this.convertValues(source["findings"], Finding);
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.updated_at = this.convertValues(source["updated_at"], null);
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

