export namespace main {
	
	export class ClientInfo {
	    sessionId: string;
	    remoteAddr: string;
	    connectedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new ClientInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.remoteAddr = source["remoteAddr"];
	        this.connectedAt = source["connectedAt"];
	    }
	}
	export class ConnectionInfo {
	    state: string;
	    serverIP: string;
	    connectedAt: string;
	    bytesSent: number;
	    bytesRecv: number;
	
	    static createFrom(source: any = {}) {
	        return new ConnectionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.serverIP = source["serverIP"];
	        this.connectedAt = source["connectedAt"];
	        this.bytesSent = source["bytesSent"];
	        this.bytesRecv = source["bytesRecv"];
	    }
	}

}

