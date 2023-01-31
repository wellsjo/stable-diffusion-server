class WS {
  constructor() {
    let url = document.location.origin+"/ws"
    url = url.replace("http", "ws")
    console.log("ws connecting", url)
    this.url = url
    this.socket = new WebSocket(url);
  }

  subscribe(uuid) {
    this.socket.send(JSON.stringify({
      "subscribe": uuid,
    }))
  }

  onOpen(fn) {
    this.socket.onopen = function(event) {
      fn(event)
    }
  }

  onMessage(fn) {
    this.socket.onmessage = (event) => fn(event)
  }
}
