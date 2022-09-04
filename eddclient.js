/**
 * @typedef {{encode(any): any, decode(any): any}} EddCodec
 */

const EddCodecJson = {
    encode(obj) {
        return JSON.stringify(obj)
    },
    decode(msg) {
        return JSON.parse(msg)
    }
}

class EddClient {
    constructor(url) {
        this.channels = {}
        this.url = url
        this.is_connected = false;
        this.codec = EddCodecJson
    }

    /**
     * @function EddClient#setCodec
     * @param {EddCodec} codec
     */
    setCodec(codec){
        this.codec = codec
    }

    start(timeout){
        if(this.is_connected){
            return
        }

        if(!this._onChanErr) {
            this._onChanErr = function (err) {
                console.log("eddwise error from server:", err)
            }
        }

        timeout = timeout ?? 5000

        const client = this
        const timer = setTimeout(function() {
            client._onChanErr("ws connection timeout")
            // client.conn?.close();
        }, timeout);
        try {
            this.conn = new WebSocket(this.url);
        } catch(err){
            this._onChanErr("error while dialing ws " + this.url + " : " + err)
            return
        }
        this.conn.onerror = (event) => {
            var reason;
            // See https://www.rfc-editor.org/rfc/rfc6455#section-7.4.1
            if (event.code === 1000)
                reason = "Normal closure, meaning that the purpose for which the connection was established has been fulfilled.";
            else if(event.code === 1001)
                reason = "An endpoint is \"going away\", such as a server going down or a browser having navigated away from a page.";
            else if(event.code === 1002)
                reason = "An endpoint is terminating the connection due to a protocol error";
            else if(event.code === 1003)
                reason = "An endpoint is terminating the connection because it has received a type of data it cannot accept (e.g., an endpoint that understands only text data MAY send this if it receives a binary message).";
            else if(event.code === 1004)
                reason = "Reserved. The specific meaning might be defined in the future.";
            else if(event.code === 1005)
                reason = "No status code was actually present.";
            else if(event.code === 1006)
                reason = "The connection was closed abnormally, e.g., without sending or receiving a Close control frame";
            else if(event.code === 1007)
                reason = "An endpoint is terminating the connection because it has received data within a message that was not consistent with the type of the message (e.g., non-UTF-8 [https://www.rfc-editor.org/rfc/rfc3629] data within a text message).";
            else if(event.code === 1008)
                reason = "An endpoint is terminating the connection because it has received a message that \"violates its policy\". This reason is given either if there is no other sutible reason, or if there is a need to hide specific details about the policy.";
            else if(event.code === 1009)
                reason = "An endpoint is terminating the connection because it has received a message that is too big for it to process.";
            else if(event.code === 1010) // Note that this status code is not used by the server, because it can fail the WebSocket handshake instead.
                reason = "An endpoint (client) is terminating the connection because it has expected the server to negotiate one or more extension, but the server didn't return them in the response message of the WebSocket handshake. <br /> Specifically, the extensions that are needed are: " + event.reason;
            else if(event.code === 1011)
                reason = "A server is terminating the connection because it encountered an unexpected condition that prevented it from fulfilling the request.";
            else if(event.code === 1015)
                reason = "The connection was closed due to a failure to perform a TLS handshake (e.g., the server certificate can't be verified).";
            else
                reason = "Unknown reason";
            this._onChanErr("error in socket communication: " + reason)
        }
        this.conn.onclose = function() { client.disconnected() }
        this.conn.onopen = function() {
            clearTimeout(timer)
            client.connected()
        }


        for (let i in this.channels) {
            if(this.channels.hasOwnProperty(i)) {
                this.channels[i].setClient(this)
            }
        }

        this.conn.onmessage = async function(evt) {
            let raw = evt.data
            if(raw instanceof Blob) {
                raw = await raw.arrayBuffer()
            }
            let data = client.codec.decode(raw)
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

    send(obj){
        if(!this.is_connected) {
            this._onChanErr('attempting to send message on inactive connection')
            return false
        }
        let enc = this.codec.encode(obj)
        this.conn.send(enc)
        return true
    }

    sendraw(msg){
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

/**
 * @typedef auth_challenge
 * @property {string[]} methods
 */

/**
 * @typedef auth_pass
 * @property {string} id
 */

/**
 * @typedef user_join
 * @property {string} id
 */

/**
 * @typedef user_left
 * @property {string} id
 */

class EddChannel {
    constructor(alias) {
        this.alias = alias
        this.client = null
        this._authChallenged = () => {
            console.log("edd auth challenge was received from server, but no handler was configured")
        }
        this._authPassed = () => {
            console.log("edd auth pass was received from server, but no handler was configured")
        }
        this._userJoin = () => {
            console.log("edd user join was received from server, but no handler was configured")
        }
        this._userLeft = () => {
            console.log("edd user left was received from server, but no handler was configured")
        }
    }

    setClient(client) {
        this.client = client
    }

    route(name, body) {
        switch (name) {
            default:
                return false
            case "edd:auth:challenge":
                this._authChallenged(body)
                break
            case "edd:auth:pass":
                this._authPassed(body)
                break
            case "edd:user:join":
                this._userJoin(body)
                break
            case "edd:user:left":
                this._userLeft(body)
                break
        }
        return true
    }

    sendAuthBasic(username, password){
        this.client.send( {channel:this.alias, name:"edd:auth:basic", body: {username:username, password:password }} );
    }

    /**
     * @callback authChallengedCb
     * @param {auth_challenge} event
     */
    /**
     * @function eddwiseChannel#authChallenged
     * @param {authChallengedCb} callback
     */
    authChallenged(callback) {
        this._authChallenged = callback
    }

    /**
     * @callback authPassedCb
     * @param {auth_pass} event
     */
    /**
     * @function eddwiseChannel#authPassed
     * @param {authPassedCb} callback
     */
    authPassed(callback) {
        this._authPassed = callback
    }

    /**
     * @callback userJoinCb
     * @param {user_join} event
     */
    /**
     * @function eddwiseChannel#userJoin
     * @param {userJoinCb} callback
     */
    userJoin(callback) {
        this._userJoin = callback
    }

    /**
     * @callback userLeftCb
     * @param {user_left} event
     */
    /**
     * @function eddwiseChannel#userLeft
     * @param {userLeftCb} callback
     */
    userLeft(callback) {
        this._userLeft = callback
    }

}

export {EddClient, EddChannel};
