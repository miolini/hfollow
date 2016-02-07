package main

import (
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/codegangsta/cli"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

func main() {
	app := cli.NewApp()
	app.Name = "hfollow"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "limit,l",
			Value: 10,
			Usage: "float64 value of timeout in seconds (for example, 10.5, 0.5)",
		},
		cli.Float64Flag{
			Name:  "timeout,t",
			Value: 15,
		},
	}
	app.Usage = "follow http(s) redirect"
	app.Action = followAction
	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}

func followAction(ctx *cli.Context) {
	addr := ctx.Args().First()
	if addr == "" {
		log.Fatal("need url")
	}
	addrP, err := url.Parse(addr)
	if err != nil {
		log.Fatalf("parse url err: %s", err)
	}
	limit := ctx.Int("limit")
	timeout := ctx.Float64("timeout")
	if timeout < 0 {
		log.Fatalf("timeout should be greater than zeo")
	}
	go func() {
		time.Sleep(time.Duration(timeout*1000) * time.Millisecond)
		log.Fatalf("timeout %.2f sec. reached", timeout)
	}()
	log.Printf("redirects limit: %d", limit)
	addrP, err = getFinalURL(addrP, limit)
	if err != nil {
		log.Fatalf("%s", err)
	}
	fmt.Println(addrP.String())
}

type followHTTPClient struct {
	http.Client
}

func (c *followHTTPClient) CheckRedirect(req *http.Request, via []*http.Request) error {
	log.Printf("redirect: %s", req.URL.String())
	return nil
}

func getFinalURL(addr *url.URL, level int) (*url.URL, error) {
	log.Printf("new request: %s", addr.String())
	if level == 0 {
		return nil, fmt.Errorf("too many redirects")
	}
	if addr.Scheme != "http" && addr.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme: %s", addr.Scheme)
	}
	client := followHTTPClient{}
	req, err := http.NewRequest("GET", addr.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request create err: %s", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request err: %s", err)
	}
	defer resp.Body.Close()
	if !strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		return resp.Request.URL, err
	}
	doc, err := goquery.NewDocumentFromResponse(resp)
	if err != nil {
		log.Fatalf("error parse html: %s", err)
	}
	redirect, ok := doc.Find("meta[http-equiv=refresh]").First().Attr("content")
	if !ok {
		return resp.Request.URL, err
	}
	sepPos := strings.Index(redirect, ";")
	if sepPos < 0 {
		return nil, fmt.Errorf("bad html meta redirect: %s", redirect)
	}
	redirect = redirect[sepPos+1:]
	sepPos = strings.Index(redirect, "=")
	if sepPos < 0 {
		return nil, fmt.Errorf("bad html meta redirect: %s", redirect)
	}
	redirect = redirect[sepPos+1:]
	addr, err = url.Parse(redirect)
	if err != nil {
		return nil, fmt.Errorf("parse html redirect err: %s", err)
	}
	return getFinalURL(addr, level-1)
}
