package reload

// Script is the JavaScript snippet that enables auto-reloading in the browser.
// Embed this into your index.html or equivalent.
// Heavily inspired by https://github.com/aarol/reload, licensed under MIT.
const Script = `
<script>
  function retry() {
    setTimeout(() => listen(true), 1000)
  }
  function listen(isRetry) {
    let protocol = location.protocol === "https:" ? "wss://" : "ws://"
    let ws = new WebSocket(protocol + location.host + "/watch")
    if (isRetry) {
      ws.onopen = () => window.location.reload()
    }
    ws.onmessage = function(msg) {
      if (msg.data === "reload") {
        window.location.reload()
      }
    }
    ws.onclose = retry
  }
  listen(false)
</script>
`
