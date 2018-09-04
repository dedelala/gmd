package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"

	"github.com/fsnotify/fsnotify"
	"github.com/google/go-github/github"
	"golang.org/x/oauth2"
)

type client struct {
	*github.Client
	repo string
	html string
	css  string
}

func (c *client) init() error {
	const url = "https://raw.githubusercontent.com/sindresorhus/github-markdown-css/gh-pages/github-markdown.css"
	rsp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer rsp.Body.Close()

	css, err := ioutil.ReadAll(rsp.Body)
	if err != nil {
		return err
	}

	c.css = string(css)
	return nil
}

func (c *client) load(path string) error {
	md, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}

	opts := &github.MarkdownOptions{
		Mode:    "markdown",
		Context: c.repo,
	}
	if c.repo != "" {
		opts.Mode = "gdm"
	}

	log.Print("rendering " + path)
	html, _, err := c.Markdown(context.Background(), string(md), opts)
	if err != nil {
		return err
	}

	c.html = html
	return nil
}

func main() {
	log.SetFlags(0)
	port := flag.Int("p", 8080, "listen on `port`")
	repo := flag.String("r", "", "render in context of `repo`")
	flag.Parse()
	path := flag.Arg(0)

	var tc *http.Client
	if t := os.Getenv("TOKEN"); t != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: t})
		tc = oauth2.NewClient(ctx, ts)
	}

	c := &client{
		Client: github.NewClient(tc),
		repo:   *repo,
	}

	if err := c.init(); err != nil {
		log.Fatal(err)
	}

	if err := c.load(path); err != nil {
		log.Fatal(err)
	}

	w, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal(err)
	}
	w.Add(path)

	go func() {
		for e := range w.Events {
			if e.Op != fsnotify.Write {
				continue
			}
			if err := c.load(e.Name); err != nil {
				log.Print(err)
			}
		}
	}()

	go func() {
		for err := range w.Errors {
			log.Print(err)
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, page, c.css, c.html)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

const page = `<!DOCTYPE html>
<html>
<head>
<style>
%s
</style>
</head>
<body>
<article class="markdown-body">
%s
</article>
</body>
</html>
`
