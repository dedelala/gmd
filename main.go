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
	"path"
	"sort"
	"strings"

	"github.com/fsnotify/fsnotify"
	"github.com/google/go-github/github"
	"golang.org/x/net/websocket"
	"golang.org/x/oauth2"
)

type renderer struct {
	*github.Client
	repo string
}

func newRenderer(token, repo, url string) (*renderer, error) {
	var tc *http.Client
	if token != "" {
		ctx := context.Background()
		ts := oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token})
		tc = oauth2.NewClient(ctx, ts)
	}
	if url != "" {
		c, err := github.NewEnterpriseClient(url, url, tc)
		return &renderer{c, repo}, err
	}
	return &renderer{github.NewClient(tc), repo}, nil
}

func (r *renderer) render(path string) (string, error) {
	md, err := ioutil.ReadFile(path)
	if err != nil {
		return "", err
	}

	opts := &github.MarkdownOptions{
		Mode:    "markdown",
		Context: r.repo,
	}
	if r.repo != "" {
		opts.Mode = "gdm"
	}

	s, _, err := r.Markdown(context.Background(), string(md), opts)
	if err != nil {
		return "", err
	}
	return s, nil
}

type server struct {
	html    map[string]string
	seen    map[string]bool
	refresh chan string
}

func newServer() *server {
	return &server{
		html:    map[string]string{},
		seen:    map[string]bool{},
		refresh: make(chan string),
	}
}

func (s *server) nav() string {
	ps := []string{}
	for p := range s.html {
		ps = append(ps, p)
	}
	sort.Strings(ps)

	const link = `<li><a href="/%s">%s</a></li>`

	n := "<nav><ul>"
	for _, p := range ps {
		l := p
		if !s.seen[p] {
			l = "<em>" + l + "</em>"
		}
		n += fmt.Sprintf(link, p, l)
	}
	n += "</ul></nav>"
	return n
}

func (s *server) sock(ws *websocket.Conn) {
	defer ws.Close()
	const article = `<article class="markdown-body">%s</article>`
	path := strings.TrimPrefix(ws.Config().Location.Path, "/sock/")

	s.seen[path] = true
	fmt.Fprintf(ws, s.nav()+article, s.html[path])
	for ev := range s.refresh {
		s.seen[ev] = false
		s.seen[path] = true
		fmt.Fprintf(ws, s.nav()+article, s.html[path])
	}
}

func watch(paths []string) (chan string, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}

	c := make(chan string)
	for _, p := range paths {
		w.Add(p)
	}

	go func() {
		for _, p := range paths {
			c <- p
		}
		for e := range w.Events {
			if e.Op != fsnotify.Write {
				continue
			}
			c <- e.Name
		}
	}()

	go func() {
		for err := range w.Errors {
			log.Print(err)
		}
	}()

	return c, nil
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
	url := flag.String("u", "", "github enterprise url")

	flag.Parse()

	css, err := style()
	if err != nil {
		log.Fatal(err)
	}

	r, err := newRenderer(os.Getenv("TOKEN"), *repo, *url)
	if err != nil {
		log.Fatal(err)
	}
	s := newServer()

	paths, err := watch(flag.Args())
	if err != nil {
		log.Fatal(err)
	}

	go func() {
		for p := range paths {
			html, err := r.render(p)
			if err != nil {
				log.Print(err)
				continue
			}
			s.html[p] = html
			s.refresh <- p
		}
	}()

	http.Handle("/sock/", websocket.Handler(s.sock))

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		if path.Ext(r.URL.Path) != ".md" {
			d, err := os.Getwd()
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			p := path.Join(d, r.URL.Path)
			f, err := os.Open(p)
			if err != nil {
				if os.IsNotExist(err) {
					http.Error(w, err.Error(), http.StatusNotFound)
					return
				}
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			defer f.Close()
			io.Copy(w, f)
		}
		fmt.Fprintf(w, page, *port, r.URL.Path, css)
	})

	log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}

const page = `<!DOCTYPE html>
<html>
    <head>
        <script>
            var sock = new WebSocket("ws://localhost:%d/sock%s");
            sock.onmessage = function (e) {
                document.getElementById("gmd-container").innerHTML = e.data;
            }
        </script>
        <style>%s</style>
    </head>
    <body>
        <div id="gmd-container"></div>
    </body>
</html>
`
