package extractor

import (
	"testing"
	b "github.com/ying32/readability"
	"net/http"
	"bytes"
	"io"
	c "github.com/mauidude/go-readability"
	d "github.com/philipjkim/goreadability"
	"github.com/PuerkitoBio/goquery"
	"strings"
)

var (
	links = []string{
		"http://www.huweihuang.com/article/source-analysis/client-go-source-analysis/",
		//"http://www.hualongxiang.com/qinggan/13865117",
		//"https://mp.weixin.qq.com/s/62KJ2mSTGoUTPsq0RjU7lg",
		//"https://ewanvalentine.io/microservices-in-golang-part-3/",
	}
)

func TestExtract(t *testing.T) {
	title,content, err := Parse("http://www.huweihuang.com/article/source-analysis/client-go-source-analysis/")
	if err != nil {
		t.Error(err)
	} else {
		t.Log(title)
		t.Log(content)
	}
}

func TestRB(t *testing.T) {
	for _, u := range links {
		doc, err := b.NewReadability(u)
		if err != nil {
			t.Error(err)
		} else {
			doc.Parse()
			t.Logf("%+v", doc.Content)
		}
		t.Log("--------------------------------")
	}
}

func TestRC(t *testing.T) {
	for _, u := range links {
		content, err := get(u)
		if err != nil {
			t.Log(err)
		} else {
			doc, err := c.NewDocument(content)
			if err != nil {
				t.Error(err)
			} else {
				t.Logf("%+v", doc.Content())
			}
		}
		t.Log("----------------------------------")
	}
}

func TestRD(t *testing.T) {
	for _, u := range links {
		o := d.NewOption()
		o.ImageRequestTimeout = 3000
		o.DescriptionAsPlainText= false

		content,err := d.Extract(u,o)
		if err != nil {
			t.Error(err)
		}else{
			t.Log(content.Description)
		}
		t.Log("----------------------------------")
	}
}

func get(u string) (string, error) {
	req, err := http.NewRequest("GET", u, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_12_6) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/61.0.3163.25 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}

	defer func() {
		if resp != nil {
			resp.Body.Close()
		}
	}()

	buf := bytes.NewBuffer(nil)
	_, err = io.Copy(buf, resp.Body)
	if err != nil {
		return "", err
	}
	return buf.String(), nil
}

func TestComment(t *testing.T) {
	html := `<html><head><title>title!</title></head><body><div><p>Some content</p><!-- this is comment --></div></body>`

	doc,err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		t.Error(err)
	}else{
		x,_ := doc.Html()
		t.Logf("%+v",x)
		y,_ := doc.Contents().Html()
		t.Logf("%+v",y)
		for i,node := range doc.Nodes{
			t.Log(i)
			t.Logf("%+v",node.Type)
		}
	}
}
