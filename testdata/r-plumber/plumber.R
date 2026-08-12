#* @apiTitle My Plumber API
#* @apiDescription A simple Plumber REST API

#* Return a greeting
#* @param name The name to greet
#* @get /hello
function(name = "World") {
  list(message = paste0("Hello, ", name, "!"))
}

#* Health check
#* @get /health
function() {
  list(status = "ok")
}
