package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/google/go-github/github"
	"golang.org/x/net/websocket"
	"golang.org/x/oauth2"
)

type client struct {
	*github.Client
	path string
	repo string
}

func (c *client) load() (string, error) {
	md, err := ioutil.ReadFile(c.path)
	if err != nil {
		return "", err
	}

	opts := &github.MarkdownOptions{
		Mode:    "markdown",
		Context: c.repo,
	}
	if c.repo != "" {
		opts.Mode = "gdm"
	}

	s, _, err := c.Markdown(context.Background(), string(md), opts)
	return s, err
}

func (c *client) sock(ws *websocket.Conn) {
	defer ws.Close()

	s, err := c.load()
	if err != nil {
		log.Print(err)
		return
	}
	io.WriteString(ws, s)

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	w.Add(c.path)
	defer w.Close()

	end := make(chan bool)
	go func() {
		for e := range w.Events {
			if e.Op != fsnotify.Write {
				continue
			}
			s, err := c.load()
			if err != nil {
				log.Print(err)
			}
			io.WriteString(ws, s)
		}
		close(end)
	}()

	go func() {
		for err := range w.Errors {
			log.Print(err)
		}
	}()

	<-end
}

func style() (string, error) {
	const url = "https://raw.githubusercontent.com/sindresorhus/github-markdown-css/gh-pages/github-markdown.css"
	rsp, err := http.Get(url)
	if err != nil {
		return "", err
	}
	defer rsp.Body.Close()

	s, err := ioutil.ReadAll(rsp.Body)
	return string(s), err
}

func main() {
	log.SetFlags(0)
	port := flag.Int("p", 8080, "listen on `port`")
	repo := flag.String("r", "", "render in context of `repo`")
	flag.Parse()

	var tc *http.Client
	if t := os.Getenv("TOKEN"); t != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: t})
		tc = oauth2.NewClient(ctx, ts)
	}

	c := &client{
		Client: github.NewClient(tc),
		path:   flag.Arg(0),
		repo:   *repo,
	}

	css, err := style()
	if err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, page, *port, css)
	})

	http.Handle("/sock", websocket.Handler(c.sock))

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

const page = `<!DOCTYPE html>
<html>
    <head>
        <script>
            var sock = new WebSocket("ws://localhost:%d/sock");
            sock.onmessage = function (e) {
                document.getElementById("gmd-container").innerHTML = e.data;
            }
        </script>
        <style>%s</style>
    </head>
    <body>
        <article id="gmd-container" class="markdown-body"></article>
    </body>
</html>
`
