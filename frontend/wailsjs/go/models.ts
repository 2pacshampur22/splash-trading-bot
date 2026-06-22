export namespace models {
	
	export class SplashTier {
	    level: number;
	    window: number;
	    isForcedPin: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SplashTier(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.level = source["level"];
	        this.window = source["window"];
	        this.isForcedPin = source["isForcedPin"];
	    }
	}
	export class EngineConfig {
	    tiers: SplashTier[];
	
	    static createFrom(source: any = {}) {
	        return new EngineConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tiers = this.convertValues(source["tiers"], SplashTier);
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
	
	export class SpreadConfig {
	    alertThresholdPct: number;
	    minVolume24h: number;
	    enableCex: boolean;
	    enableDex: boolean;
	    pollingIntervalMs: number;
	
	    static createFrom(source: any = {}) {
	        return new SpreadConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.alertThresholdPct = source["alertThresholdPct"];
	        this.minVolume24h = source["minVolume24h"];
	        this.enableCex = source["enableCex"];
	        this.enableDex = source["enableDex"];
	        this.pollingIntervalMs = source["pollingIntervalMs"];
	    }
	}
	export class SpreadSignal {
	    symbol: string;
	    buyExchange: string;
	    sellExchange: string;
	    buyPrice: number;
	    sellPrice: number;
	    spreadPct: number;
	    volume24h: number;
	    source: string;
	    chain: string;
	    timestamp: string;
	    isAlert: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SpreadSignal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.symbol = source["symbol"];
	        this.buyExchange = source["buyExchange"];
	        this.sellExchange = source["sellExchange"];
	        this.buyPrice = source["buyPrice"];
	        this.sellPrice = source["sellPrice"];
	        this.spreadPct = source["spreadPct"];
	        this.volume24h = source["volume24h"];
	        this.source = source["source"];
	        this.chain = source["chain"];
	        this.timestamp = source["timestamp"];
	        this.isAlert = source["isAlert"];
	    }
	}

}

