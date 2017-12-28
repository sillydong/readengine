package main

import (
	"github.com/sillydong/readengine/extractor"
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
	"github.com/sillydong/goczd/gotime"
	"github.com/boltdb/bolt"
	"encoding/json"
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
			Name:      "search",
			Aliases:   []string{"s"},
			Usage:     "search in read history",
			Action:    search,
			ArgsUsage: "keyword",
		},
		{
			Name:    "rebuild",
			Aliases: []string{"r"},
			Usage:   "rebuild index from database",
			Action:  rebuild,
		},
	}
	app.Run(os.Args)
}

var (
	conf  Conf
	idx   bleve.Index
	jieba *gojieba.Jieba
	db    *bolt.DB
)

type Conf struct {
	Dict     string `yaml:"dict"`
	Hmm      string `yaml:"hmm"`
	UserDict string `yaml:"userdict"`
	Idf      string `yaml:"idf"`
	Stop     string `yaml:"stop"`
	Store    string `yaml:"store"`
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

	//init index
	jieba = gojieba.NewJieba(conf.Dict, conf.Hmm, conf.UserDict, conf.Idf, conf.Stop)
	indexpath := path.Join(conf.Store, "index")
	idx, err = bleve.Open(indexpath)
	if err == bleve.ErrorIndexPathDoesNotExist {
		mapping := bleve.NewIndexMapping()
		if err := mapping.AddCustomTokenizer("gojieba", map[string]interface{}{
			"dictpath":     conf.Dict,
			"hmmpath":      conf.Hmm,
			"userdictpath": conf.UserDict,
			"idf":          conf.Idf,
			"stop_words":   conf.Stop,
			"type":         "gojieba",
		}); err != nil {
			logrus.Fatal(err)
		}
		if err := mapping.AddCustomAnalyzer("gojieba", map[string]interface{}{
			"type":      "gojieba",
			"tokenizer": "gojieba",
		}); err != nil {
			panic(err)
		}
		mapping.DefaultAnalyzer = "gojieba"
		docmapping := bleve.NewDocumentMapping()
		fieldidmapping := bleve.NewNumericFieldMapping()
		docmapping.AddFieldMappingsAt("Id", fieldidmapping)
		fieldtitlemapping := bleve.NewTextFieldMapping()
		docmapping.AddFieldMappingsAt("Title", fieldtitlemapping)
		fieldcontentmapping := bleve.NewTextFieldMapping()
		docmapping.AddFieldMappingsAt("Content", fieldcontentmapping)
		idx, err = bleve.New(indexpath, mapping)
		if err != nil {
			logrus.Fatal(err)
		}
	} else if err != nil {
		logrus.Fatal(err)
	}

	//init db
	datapath := path.Join(conf.Store, "data.db")
	db, err = bolt.Open(datapath, 0600, nil)
	if err != nil {
		logrus.Fatal(err)
	}
	err = db.Update(func(tx *bolt.Tx) error {
		_,err := tx.CreateBucketIfNotExists([]byte("readengine"))
		return err
	})
	if err != nil {
		db.Close()
		logrus.Fatal(err)
	}
}

func close_engine() {
	logrus.Info("engine close")
	idx.Close()
	jieba.Free()
	db.Close()
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
			Id:      strconv.FormatInt(time.Now().Unix(), 10),
			Src:     url,
			Title:   title,
			Content: content,
		}
		//save to db
		db.Update(func(tx *bolt.Tx) error {
			docbytes, err := json.Marshal(doc)
			if err != nil {
				return err
			}
			return tx.Bucket([]byte("readengine")).Put([]byte(doc.Id), docbytes)
		})

		//save to index
		if err := idx.Index(doc.Id, doc); err != nil {
			logrus.Error(err)
		} else {
			logrus.Infof("indexed %v", title)
			c, _ := idx.DocCount()
			logrus.Infof("index size: %v", c)
		}
	}

	return nil
}

func search(c *cli.Context) error {
	if c.NArg() == 0 {
		return cli.ShowCommandHelp(c, "search")
	}
	init_engine(c)
	defer close_engine()

	keyword := c.Args().First()

	req := bleve.NewSearchRequest(bleve.NewQueryStringQuery("Title:" + keyword + " Content:" + keyword))
	req.Fields = []string{"Id", "Src", "Title"}
	req.Highlight = bleve.NewHighlight()

	res, err := idx.Search(req)
	if err != nil {
		logrus.Error(err)
	} else {
		if res.Total > 0 {
			logrus.Infof("找到 %v 条结果", res.Total)
			for _, doc := range res.Hits {
				addtime, _ := strconv.Atoi(doc.ID)
				logrus.Infof("[%v]title: %v\n\t\tsrc: %v", gotime.TimeToStr(int64(addtime), gotime.FORMAT_YYYY_MM_DD_HH_II_SS), doc.Fields["Title"], doc.Fields["Src"])
			}
		} else {
			logrus.Info("未找到结果")
		}
	}
	return nil
}

func rebuild(c *cli.Context) error {
	init_engine(c)
	defer close_engine()

	db.View(func(tx *bolt.Tx) error {
		b := tx.Bucket([]byte("readengine"))
		c := b.Cursor()
		for k, v := c.First(); k != nil; k, v = c.Next() {
			doc := Doc{}
			if err := json.Unmarshal(v, &doc); err != nil {
				logrus.Error(err)
			} else {
				logrus.Infof("indexing %v", doc.Src)
				if err := idx.Index(doc.Id, doc); err != nil {
					logrus.Error(err)
				}
			}
		}
		return nil
	})

	count, _ := idx.DocCount()
	logrus.Infof("rebuild index finished, index size: %v", count)

	return nil
}

type Doc struct {
	Id      string
	Src     string
	Title   string
	Content string
}
