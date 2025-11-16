# reload

Auto-reload browser when assets change

# How to use

1. Embed the JS script into your index.html or equivalent, for example,
   using `fmt.Sprintf`:

   ```go
   var indexHTML = fmt.Sprintf(
   	`<!doctype html>
   <html lang="en" class="h-full bg-gray-100">
   <head>
      <meta charset="UTF-8" />
      <meta name="viewport" content="width=device-width, initial-scale=1.0" />
      <meta http-equiv="X-UA-Compatible" content="ie=edge" />
      <title>My Website</title>
      <link rel="stylesheet" href="index.css" />
      <link rel="icon" href="favicon.svg" />
      <script src="index.js" defer></script>
   </head>
   <body class="h-full">
      <div id="root" class="h-full"></div>
   </body>
   %s
   </html>
   `, reload.Script)
   func ServeIndexHTML(w http.ResponseWriter, r *http.Request) {
   	w.Write([]byte(indexHTML))
   }
   ```

2. Wrap your normal HTML handler in the middleware:

   ```go
   var srvHandler http.Handler // Handler to serve your HTML
   var log *slog.Logger // Your slog logger
   srvHandler, err = reload.NewMiddleware(
   	// Pass in the old HTML handler.
   	srvHandler,
   	// Pass in the directory you want to watch.
   	// When a file in this directory changes, the browser will reload.
   	serveDir,
   	// Optionally, pass in a logger to be notified of detected file changes.
   	log,
   )
   if err != nil {
   	return fmt.Errorf("failed to create reload middleware: %w", err)
   }
   ```

For a full example of this in use, see https://github.com/catzkorn/trail-tools.

# Inspiration

Greatly based on https://github.com/aarol/reload, but with some different dependencies and design choices.
