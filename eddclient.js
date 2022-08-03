class EddClient {
    constructor(url) {
        this.channels = {}
        this.url = url
        this.is_connected = false;
    }

    start(){
        if(this.is_connected){
            return
        }
        const client = this
        this.conn = new WebSocket(this.url);
        this.conn.onclose = function() { client.disconnected() }
        this.conn.onopen = function() { client.connected() }
        this.conn.onerror = this.error
        this._onChanErr = function(err){
            console.log("eddwise error from server:", err)
        }

        for (let i in this.channels) {
            if(this.channels.hasOwnProperty(i)) {
                this.channels[i].setConn(this.conn)
            }
        }

        this.conn.onmessage = function(evt) {
            let data = JSON.parse(evt.data)
            if(data.channel === "errors") {
                client._onChanErr(data.body)
                return
            }
            if(!client.channels.hasOwnProperty(data.channel)){
                console.log("received message from unknown channel ", data)
            }
            const ch = client.channels[data.channel]
            ch.route(data.name, data.body)
        }
    }

    stop(){
        if(this.is_connected) {
            this.is_connected = false;
            this.conn.close();
        }
    }

    register(channel) {
        this.channels[channel.getAlias()] = channel
        if(this.conn) {
            channel.setConn(this.conn)
        }

    }

    connected(){
        this.is_connected = true;
        for (let i in this.channels) {
            if(this.channels.hasOwnProperty(i)) {
                if(this.channels[i]._connectedFn != null) {
                    this.channels[i]._connectedFn()
                }
            }
        }
    }

    disconnected(){
        this.is_connected = false;
        for (let i in this.channels) {
            if(this.channels.hasOwnProperty(i)) {
                if(this.channels[i]._disconnectedFn != null) {
                    this.channels[i]._disconnectedFn()
                }
            }
        }
    }

    error(err){
        console.log("error in socket communication:", err)
    }

    /**
     * @callback onChanErrCb
     * @param {string} error
     */
    /**
     * @function EddClient#onChanErr
     * @param {onChanErrCb} callback
     */
    onChanErr(callback) {
        this._onChanErr = callback
    }
}

export {EddClient};
