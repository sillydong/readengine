package extractor

import (
	"bytes"
	"crypto/tls"
	"errors"
	"io/ioutil"
	"net/http"
	"net/url"
	"regexp"
	"strings"

	"golang.org/x/text/encoding"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
	"time"
	"compress/gzip"
	"compress/flate"
	"io"
	"github.com/PuerkitoBio/goquery"
)

var (
	client *http.Client
	header http.Header
	o      *Option
)

func init() {
	client = &http.Client{
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: false,
			},
		},
		Timeout: 30 * time.Second,
	}
	header = http.Header{}
	header.Set("User-Agent", "Mozilla/5.0 (Windows NT 6.1; WOW64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/54.0.2840.71 Safari/537.36")
	header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")
	header.Set("Accept-Encoding", "gzip, deflate")
	header.Set("Accept-Language", "zh-CN,zh;q=0.8")
	header.Set("Connection", "keep-alive")
	header.Set("Cache-Control", "max-age=0")

	o = NewOption()
	o.ImageRequestTimeout = 3000
	o.DescriptionAsPlainText = true
	o.RemoveEmptyNodes = true
}

func Parse(src string) (string, string, error) {
	//get page content
	page, err := request(src)
	if err != nil {
		return "", "", err
	}

	//replace comment blocks
	regx, _ := regexp.Compile(`<!--.+-->`)
	page = regx.ReplaceAll(page, nil)

	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(page))
	if err != nil {
		return "", "", err
	}

	//extract
	content, err := ExtractFromDocument(doc, src, o)
	if err != nil {
		return "", "", err
	}
	return content.Title, content.Description, nil
}

func request(rawUrl string) ([]byte, error) {
	u, err := url.Parse(rawUrl)
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequest(http.MethodGet, u.String(), nil)
	if err != nil {
		return nil, err
	}
	req.Header = header
	req.Header.Set("Host", u.Host)
	req.Header.Set("Referer", u.Scheme+"://"+u.Host)

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	contentEncoding := strings.Trim(strings.ToLower(resp.Header.Get("Content-Encoding")), " ")
	var x io.Reader
	if contentEncoding == "gzip" {
		x, err = gzip.NewReader(resp.Body)
		if err != nil {
			return nil, err
		}
	} else if contentEncoding == "deflate" {
		x = flate.NewReader(resp.Body)
	} else {
		x = resp.Body
	}

	bs, err := ioutil.ReadAll(x)
	if err != nil {
		return nil, err
	}
	charset := "utf8"
	regx, _ := regexp.Compile(`<meta.*?charset=["']?([a-zA-Z0-9-]+)["']?`)
	tmp := regx.FindSubmatch(bs)
	if len(tmp) == 2 {
		charset = strings.ToLower(string(tmp[1]))
	}
	if charset != "utf8" && charset != "utf-8" {
		var decoder *encoding.Decoder
		switch charset {
		case "gbk":
			decoder = simplifiedchinese.GBK.NewDecoder()
		case "gb2312":
			decoder = simplifiedchinese.HZGB2312.NewDecoder()
		case "gb18030":
			decoder = simplifiedchinese.GB18030.NewDecoder()
		default:
			return nil, errors.New(charset + " not support")
		}
		trans := transform.NewReader(bytes.NewReader(bs), decoder)
		return ioutil.ReadAll(trans)
	} else {
		return bs, nil
	}
}
