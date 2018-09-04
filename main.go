package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"net/http"

	"github.com/google/go-github/github"
)

type client struct {
	*github.Client
	src  string
	ctx  string
	html string
}

func (c *client) load() error {
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

	if err := c.load(); err != nil {
		log.Fatal(err)
	}

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, c.html)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *prt), nil))
}
