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

        if(!this._onChanErr) {
            this._onChanErr = function (err) {
                console.log("eddwise error from server:", err)
            }
        }

        const client = this
        try {
            this.conn = new WebSocket(this.url);
        } catch(err){
            this._onChanErr("error while dialing ws " + this.url + " : " + err)
        }
        this.conn.onerror = (event) => {
            this._onChanErr("error in socket communication")
        }
        this.conn.onclose = function() { client.disconnected() }
        this.conn.onopen = function() { client.connected() }


        for (let i in this.channels) {
            if(this.channels.hasOwnProperty(i)) {
                this.channels[i].setClient(this)
            }
        }

        this.conn.onmessage = function(evt) {
            let data = JSON.parse(evt.data)
            if(data.channel === "errors") {
                client._onChanErr(data.body)
                return
            }
            if(!client.channels.hasOwnProperty(data.channel)){
                client._onChanErr("received message from unknown channel, see console for details")
                console.log("received message from unknown channel, see console for details", data)
                return
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
            channel.setClient(this)
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


    send(msg){
        if(!this.is_connected) {
            this._onChanErr('attempting to send message on inactive connection')
            return false
        }
        this.conn.send(msg);
        return true
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
