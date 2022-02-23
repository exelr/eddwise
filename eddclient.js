class EddClient {
    constructor(url) {
        this.channels = {}
        this.url = url
    }

    start(){
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

    register(channel) {
        this.channels[channel.getAlias()] = channel
        channel.setConn(this.conn)
    }

    connected(){
        for (let i in this.channels) {
            if(this.channels.hasOwnProperty(i)) {
                if(this.channels[i]._connectedFn != null) {
                    this.channels[i]._connectedFn()
                }
            }
        }
    }

    disconnected(){
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
