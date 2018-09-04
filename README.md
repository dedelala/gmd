# gmd

Render local markdown using the GitHub API. View in browser.

Local files are watched for changes with [fsnotify](https://github.com/fsnotify/fsnotify),
the page is updated by websocket.

**Usage**

```
TOKEN="..." gmd README.md
```

**Options**

```
Usage of gmd:
  -p port
    	listen on port (default 8080)
  -r repo
    	render in context of repo
```

Credit to [sindresorhus](https://github.com/sindresorhus/github-markdown-css)
for the stylesheet.

