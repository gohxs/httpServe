(function (window, WsRpc) {
  var cli = new WsRpc()

  cli.export({
    'reload': function (result) {
      result()
      console.log('Reload should happen')
      window.location.reload()
    }
  })
  cli.connect('ws://' + window.location.host + '/.httpServe/_reload/ws')
  cli.onopen = function () {
    cli.call('watch', window.location.pathname).then(function (res) {
      console.log('Watching:', res)
    })
  }
})(window, WsRpc)
