package main

import (
	"fmt"
	"github.com/sillydong/readengine/extractor"
	"github.com/sillydong/readengine/filehandler"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"time"
	"github.com/yanyiwu/gojieba"
	"github.com/blevesearch/bleve"
	_ "github.com/yanyiwu/gojieba/bleve"
	"strconv"
)

func main() {
	app := cli.NewApp()
	app.Name = "ReadEngine"
	app.Usage = "INDEX whatever you read and SEARCH later"
	app.Flags = []cli.Flag{
		cli.StringFlag{
			Name:  "config, c",
			Usage: "manually set config file",
			Value: "./config.yaml",
		},
	}
	app.Commands = []cli.Command{
		{
			Name:      "url",
			Aliases:   []string{"u"},
			Usage:     "read content from url and index the main content",
			Action:    index_url,
			ArgsUsage: "absolute url",
		},
		{
			Name:    "file",
			Aliases: []string{"f"},
			Usage:   "read content from file and index the whole file content",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "type,t",
					Usage: "manully set file type",
					Value: "",
				},
			},
			Action:    index_file,
			ArgsUsage: "filepath",
		},
		{
			Name:      "search",
			Aliases:   []string{"s"},
			Usage:     "search in read history",
			Action:    search,
			ArgsUsage: "keyword",
		},
		{
			Name:    "web",
			Aliases: []string{"w"},
			Usage:   "run web server for user to manage data on web",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "host,h",
					Usage: "bind to host",
					Value: "127.0.0.1",
				},
				cli.IntFlag{
					Name:  "port,p",
					Usage: "bind to port",
					Value: 8090,
				},
			},
			Action: web,
		},
	}
	app.Run(os.Args)
}

var (
	conf Conf
	idx bleve.Index
	jieba *gojieba.Jieba
)

type Conf struct {
	Dict      string `yaml:"dict"`
	Hmm string `yaml:"hmm"`
	UserDict string `yaml:"userdict"`
	Idf string `yaml:"idf"`
	Stop string `yaml:"stop"`
	Store     string `yaml:"store"`
}

func init_engine(c *cli.Context) {
	runtimepath := path.Dir(os.Args[0])

	configpath := c.GlobalString("config")
	if !path.IsAbs(configpath) {
		configpath = path.Join(runtimepath, configpath)
	}

	content, err := ioutil.ReadFile(configpath)
	if err != nil {
		logrus.Fatal(err)
	}
	err = yaml.Unmarshal(content, &conf)
	if err != nil {
		logrus.Fatal(err)
	}

	if conf.Store == "" {
		conf.Store = path.Join(runtimepath, "store")
	} else if !path.IsAbs(conf.Store) {
		conf.Store = path.Join(runtimepath, conf.Store)
	}

	jieba = gojieba.NewJieba(conf.Dict,conf.Hmm,conf.UserDict,conf.Idf,conf.Stop)
	idx ,err = bleve.Open(conf.Store)
	if err == bleve.ErrorIndexPathDoesNotExist {
		mapping := bleve.NewIndexMapping()
		if err := mapping.AddCustomTokenizer("gojieba",map[string]interface{}{
			"dictpath":     conf.Dict,
			"hmmpath":      conf.Hmm,
			"userdictpath": conf.UserDict,
			"idf":          conf.Idf,
			"stop_words":   conf.Stop,
			"type":         "gojieba",
		});err != nil {
			logrus.Fatal(err)
		}
		if err := mapping.AddCustomAnalyzer("gojieba",map[string]interface{}{
			"type":      "gojieba",
			"tokenizer": "gojieba",
		}); err != nil {
			panic(err)
		}
		mapping.DefaultAnalyzer="gojieba"
		docmapping := bleve.NewDocumentMapping()
		fieldidmapping := bleve.NewNumericFieldMapping()
		docmapping.AddFieldMappingsAt("Id",fieldidmapping)
		fieldtitlemapping := bleve.NewTextFieldMapping()
		docmapping.AddFieldMappingsAt("Title",fieldtitlemapping)
		fieldcontentmapping := bleve.NewTextFieldMapping()
		docmapping.AddFieldMappingsAt("Content",fieldcontentmapping)
		idx,err = bleve.New(conf.Store,mapping)
		if err != nil {
			logrus.Fatal(err)
		}
	}
}

func close_engine() {
	logrus.Info("engine close")
	idx.Close()
	jieba.Free()
}

func index_url(c *cli.Context) error {
	if c.NArg() == 0 {
		return cli.ShowCommandHelp(c, "url")
	}
	init_engine(c)
	defer close_engine()

	url := c.Args().First()
	logrus.Infof("indexing %v", url)

	title, content, err := extractor.Parse(url)
	if err != nil {
		logrus.Error(err)
	} else {
		doc := &Doc{
			Id:strconv.FormatInt(time.Now().Unix(),10),
			Src:url,
			Title:title,
			Content:content,
		}
		if err := idx.Index(doc.Id,doc);err !=nil{
			logrus.Error(err)
		}else{
			logrus.Infof("indexed %v", title)
			c ,_ := idx.DocCount()
			logrus.Infof("index size: %v", c)
		}
	}

	return nil
}

func index_file(c *cli.Context) error {
	if c.NArg() == 0 {
		return cli.ShowCommandHelp(c, "file")
	}
	init_engine(c)
	defer close_engine()

	filepath := c.Args().First()
	logrus.Infof("indexing %v", filepath)

	title,content, err := filehandler.Parse(filepath)
	if err != nil {
		logrus.Error(err)
	}

	fmt.Println(title)
	fmt.Println(content)
	return nil
}

func search(c *cli.Context) error {
	if c.NArg() == 0 {
		return cli.ShowCommandHelp(c, "search")
	}
	init_engine(c)
	defer close_engine()

	keyword := c.Args().First()

	req := bleve.NewSearchRequest(bleve.NewQueryStringQuery("Title:"+keyword+" Content:"+keyword))
	req.Fields=[]string{"Id","Url","Title"}
	req.Highlight = bleve.NewHighlight()

	res,err := idx.Search(req)
	if err != nil {
		logrus.Error(err)
	}else{
		if res.Total>0{
			logrus.Infof("找到 %v 条结果", res.Total)
			for _, doc := range res.Hits {
				logrus.Infof("[%v]title:%v\n\tsrc%v",doc.ID,doc.Fields["Title"],doc.Fields["Url"])
			}
		}else{
			logrus.Info("未找到结果")
		}
	}
	return nil
}

func web(c *cli.Context) error {
	init_engine(c)
	fmt.Println("web")
	return nil
}

type Doc struct{
	Id string
	Src string
	Title string
	Content string
}
