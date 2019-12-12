package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path"
	"path/filepath"
	"strconv"
	"time"

	"github.com/blevesearch/bleve"
	"github.com/boltdb/bolt"
	"github.com/sillydong/goczd/gotime"
	"github.com/sillydong/readengine/extractor"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"github.com/yanyiwu/gojieba"
	_ "github.com/yanyiwu/gojieba/bleve"
	"gopkg.in/yaml.v2"
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
			Name:      "del",
			Aliases:   []string{"d"},
			Usage:     "delete content from index by id",
			Action:    del_id,
			ArgsUsage: "doc id",
		},
		{
			Name:      "read",
			Aliases:   []string{"r"},
			Usage:     "read content from index by id",
			Action:    read_id,
			ArgsUsage: "doc id",
		},
		{
			Name:      "search",
			Aliases:   []string{"s"},
			Usage:     "search in read history",
			Action:    search,
			ArgsUsage: "keyword",
		},
		{
			Name:    "history",
			Aliases: []string{"hi"},
			Usage:   "show all indexed urls",
			Action:  history,
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
	user, err := user.Current()
	if err != nil {
		logrus.Fatal(err)
	}

	configdir := filepath.Join(user.HomeDir, ".readengine")
	configfile := filepath.Join(configdir, "config.yaml")

	content, err := ioutil.ReadFile(configfile)
	if err != nil {
		logrus.Fatal(err)
	}
	err = yaml.Unmarshal(content, &conf)
	if err != nil {
		logrus.Fatal(err)
	}

	if conf.Dict == "" || conf.Hmm == "" || conf.UserDict == "" || conf.Idf == "" || conf.Stop == "" {
		logrus.Fatal("missing configuration for segment")
	}

	if !path.IsAbs(conf.Dict) {
		conf.Dict = path.Join(configdir, conf.Dict)
	}
	if !path.IsAbs(conf.Hmm) {
		conf.Hmm = path.Join(configdir, conf.Hmm)
	}
	if !path.IsAbs(conf.UserDict) {
		conf.UserDict = path.Join(configdir, conf.UserDict)
	}
	if !path.IsAbs(conf.Idf) {
		conf.Idf = path.Join(configdir, conf.Idf)
	}
	if !path.IsAbs(conf.Stop) {
		conf.Stop = path.Join(configdir, conf.Stop)
	}

	if conf.Store == "" {
		conf.Store = path.Join(configdir, "store")
	} else if !path.IsAbs(conf.Store) {
		conf.Store = path.Join(configdir, conf.Store)
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
		_, err := tx.CreateBucketIfNotExists([]byte("readengine"))
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

func del_id(c *cli.Context) error {
	if c.NArg() == 0 {
		return cli.ShowCommandHelp(c, "id")
	}
	init_engine(c)
	defer close_engine()

	id := c.Args().First()
	logrus.Infof("deleting index by id %v", id)

	if err := idx.Delete(id); err != nil {
		logrus.Error(err)
		return err
	}

	return nil
}

func read_id(c *cli.Context) error {
	if c.NArg() == 0 {
		return cli.ShowCommandHelp(c, "id")
	}
	init_engine(c)
	defer close_engine()

	id := c.Args().First()
	logrus.Infof("reading index by id %v", id)

	doc, err := idx.Document(id)
	if err != nil {
		logrus.Error(err)
		return err
	}

	if doc == nil {
		logrus.Error("未找到数据")
		return nil
	}

	for _, field := range doc.Fields {
		fmt.Printf("%s: %s", field.Name(), string(field.Value()))
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
				logrus.Infof("[%s][%v]title: %v\n\t\tsrc: %v", doc.ID, gotime.TimeToStr(int64(addtime), gotime.FORMAT_YYYY_MM_DD_HH_II_SS), doc.Fields["Title"], doc.Fields["Src"])
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

func history(c *cli.Context) error {
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
				addtime, _ := strconv.Atoi(doc.Id)
				logrus.Infof("[%v]title: %v\n\t\tsrc: %v", gotime.TimeToStr(int64(addtime), gotime.FORMAT_YYYY_MM_DD_HH_II_SS), doc.Title, doc.Src)
			}
		}
		return nil
	})

	count, _ := idx.DocCount()
	logrus.Infof("index size: %v", count)

	return nil
}

type Doc struct {
	Id      string
	Src     string
	Title   string
	Content string
}
