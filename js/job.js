document.addEventListener("DOMContentLoaded", function(){
  const ws = new WS()

  ws.onOpen (() => {
    console.log("socket connected", ws.url)
    if (!_jobArchived) {
      ws.subscribe(_uuid)
    }
  })

  ws.onMessage ((event) => {
    const wsJSON = JSON.parse(event.data)
    const cmd = Object.keys(wsJSON)[0]
    const arg = wsJSON[cmd]

    switch (cmd) {
      case "job":

        switch (arg) {
          case "running":
            updateStatus("creating image")
            startTimer()
            break

          case "done":
            stopTimer()
            showImage()
            updateStatus("done")
            break

          default:
            throw new Error("invalid job command")
        }

        break

      case "subscribed":
        updateStatus("subscribed to updates")
        break

      default:
        console.log("no command found for ws message", cmd)
    }
  })
})

function showImage() {
  let img = document.createElement("img")
  img.src = _imgURL
  document.body.appendChild(img)
}

function updateStatus(status) {
  const element = document.getElementById("job-status")
  element.innerHTML = status
}

let intervalID = 0

function startTimer() {
  console.log("start timer")
  const element = document.getElementById("job-status")
  let timer = 1
  intervalID = setInterval(() => {
    element.textContent = "job is running (" + timer + "s)"
    timer++
  }, 1000)
}

function stopTimer() {
  clearInterval(intervalID)
}
