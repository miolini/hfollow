package main

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/PuerkitoBio/goquery"
	"github.com/codegangsta/cli"
	"io"
	"log"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"
)

const userAgent = "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/41.0.2227.0 Safari/537.36"

var errStopRedirect = errors.New("Stop Redirect")
var flDebug bool

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
		cli.BoolFlag{
			Name:        "debug,d",
			Destination: &flDebug,
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
	if flDebug {
		log.Printf("redirects limit: %d", limit)
	}
	jar, err := cookiejar.New(nil)
	if err != nil {
		log.Fatalf("jar new err: %s", err)
	}
	if flDebug {
		log.Printf("target addr: %s", addrP.String())
	}
	addrP, err = getFinalURL(jar, addrP, limit)
	if err != nil {
		log.Fatalf("%s", err)
	}
	fmt.Println(addrP.String())
}

func getFinalURL(jar *cookiejar.Jar, addr *url.URL, level int) (*url.URL, error) {
	if flDebug {
		log.Printf("new request (level %d): %s", level, addr.String())
	}
	if level == 0 {
		return nil, fmt.Errorf("too many redirects")
	}
	if addr.Scheme != "http" && addr.Scheme != "https" {
		return nil, fmt.Errorf("unsupported url scheme: %s", addr.Scheme)
	}
	req, err := http.NewRequest("GET", addr.String(), nil)
	if err != nil {
		return nil, fmt.Errorf("request create err: %s", err)
	}
	cookies := jar.Cookies(addr)
	for _, cookie := range cookies {
		if flDebug {
			log.Printf("add cookie: %s", cookie.String())
		}
		req.AddCookie(cookie)
	}
	req.Header.Set("User-Agent", userAgent)
	resp, err := http.DefaultTransport.RoundTrip(req)
	if err != nil {
		if uErr, ok := err.(*url.Error); !ok || uErr.Err != errStopRedirect {
			return nil, fmt.Errorf("request err: %s", err)
		}
	}
	defer resp.Body.Close()
	ct := strings.ToLower(resp.Header.Get("Content-Type"))
	if flDebug {
		log.Printf("response code: %d, content-type %s", resp.StatusCode, ct)
	}
	jar.SetCookies(resp.Request.URL, resp.Cookies())
	if resp.StatusCode == 301 || resp.StatusCode == 302 || resp.StatusCode == 303 || resp.StatusCode == 307 {
		location := resp.Header.Get("Location")
		addr, err = url.Parse(location)
		if err != nil {
			return nil, err
		}
		return getFinalURL(jar, addr, level-1)
	}
	if !strings.HasPrefix(ct, "text/html") {
		if flDebug {
			log.Printf("not html content-type: %s", ct)
		}
		return resp.Request.URL, err
	}
	buf := &bytes.Buffer{}
	_, err = io.CopyN(buf, resp.Body, 1024*1024*10)
	if err != nil && err != io.EOF {
		return nil, err
	}
	data := bytes.ToLower(buf.Bytes())
	if flDebug {
		log.Printf("html: %s", string(data))
	}
	metaRedirect, err := findMetaRedirect(data)
	if err != nil {
		return nil, fmt.Errorf("meta redirect err: %s", err)
	}
	if metaRedirect == nil {
		return addr, nil
	}
	if flDebug {
		log.Printf("html meta redirect: %s", metaRedirect)
	}
	return getFinalURL(jar, metaRedirect, level-1)
}

var patternMetaRefresh = regexp.MustCompile("<meta[\\s]+http-equiv=[\"']*refresh[\"']*[\\s]+content=[\"']*[\\d]+;url=(.+?)[\"']*[\\s>]")

func findMetaRedirect(data []byte) (*url.URL, error) {
	u, err := findMetaByGoquery(bytes.NewBuffer(data))
	if err != nil {
		return nil, err
	}
	if u != nil {
		return u, nil
	}
	b := patternMetaRefresh.FindSubmatch(data)
	if len(b) != 2 {
		return nil, nil
	}
	addr := string(b[1])
	if addr[0] == '\'' {
		addr = addr[1:]
	}
	return url.Parse(addr)
}

func findMetaByGoquery(data *bytes.Buffer) (*url.URL, error) {
	doc, err := goquery.NewDocumentFromReader(data)
	if err != nil {
		return nil, fmt.Errorf("error parse html: %s", err)
	}
	redirect, ok := doc.Find("meta[http-equiv=refresh]").First().Attr("content")
	if !ok {
		return nil, nil
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
	return url.Parse(redirect)
}
