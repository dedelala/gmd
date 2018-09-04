package main

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/google/go-github/github"
)

type client struct {
	*github.Client
	src  string
	ctx  string
	html string
	css  string
	mod  time.Time
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

func (c *client) load() error {
	s, err := os.Stat(c.src)
	if err != nil {
		return err
	}
	if c.mod == s.ModTime() {
		return nil
	}
	c.mod = s.ModTime()

	md, err := ioutil.ReadFile(c.src)
	if err != nil {
		return err
	}

	opts := &github.MarkdownOptions{
		Mode:    "markdown",
		Context: c.ctx,
	}
	if c.ctx != "" {
		opts.Mode = "gdm"
	}

	log.Print("rendering " + c.src)
	html, _, err := c.Markdown(context.Background(), string(md), opts)
	if err != nil {
		return err
	}

	c.html = html
	return nil
}

func main() {
	log.SetFlags(0)
	prt := flag.Int("p", 8080, "listen on `port`")
	ctx := flag.String("r", "", "render in context of `repo`")
	flag.Parse()

	c := &client{
		Client: github.NewClient(nil),
		src:    flag.Arg(0),
		ctx:    *ctx,
	}

	if err := c.init(); err != nil {
		log.Fatal(err)
	}

	go func() {
		t := time.NewTicker(100 * time.Millisecond)
		for range t.C {
			if err := c.load(); err != nil {
				log.Fatal(err)
			}
		}
	}()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, page, c.css, c.html)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *prt), nil))
}

const page = `<html>
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
