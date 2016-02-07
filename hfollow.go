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
)

func main() {
	app := cli.NewApp()
	app.Name = "hfollow"
	app.Flags = []cli.Flag{
		cli.IntFlag{
			Name:  "limit,l",
			Value: 10,
		},
	}
	app.Usage = "follow http(s) redirect"
	app.Action = followAction
	app.Run(os.Args)
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
	log.Printf("redirects limit: %d", limit)
	addrP, err = getFinalUrl(addrP, limit)
	if err != nil {
		log.Fatalf("%s", err)
	}
	fmt.Println(addrP.String())
}

type FollowHttpClient struct {
	http.Client
}

func (c *FollowHttpClient) CheckRedirect(req *http.Request, via []*http.Request) error {
	log.Printf("redirect: %s", req.URL.String())
	return nil
}

func getFinalUrl(addr *url.URL, level int) (*url.URL, error) {
	log.Printf("new request: %s", addr.String())
	if level == 0 {
		return nil, fmt.Errorf("too many redirects")
	}
	if addr.Scheme != "http" && addr.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme: %S", addr.Scheme)
	}
	client := FollowHttpClient{}
	req, err := http.NewRequest("GET", addr.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request create err: %s", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request err: %s", err)
	}
	if strings.HasPrefix(strings.ToLower(resp.Header.Get("Content-Type")), "text/html") {
		doc, err := goquery.NewDocumentFromResponse(resp)
		if err != nil {
			log.Fatalf("error parse html: %s", err)
		}
		redirect, ok := doc.Find("meta[http-equiv=refresh]").First().Attr("content")
		if ok {
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
			return getFinalUrl(addr, level-1)
		}
	}
	defer resp.Body.Close()
	return resp.Request.URL, nil
}
