package main

import (
	"fmt"
	"github.com/huichen/wukong/engine"
	"github.com/huichen/wukong/types"
	"github.com/sillydong/readengine/extractor"
	"github.com/sillydong/readengine/filehandler"
	"github.com/sirupsen/logrus"
	"github.com/urfave/cli"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"path"
	"time"
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
	e    = engine.Engine{}
	conf Conf
)

type Conf struct {
	Dict      string `yaml:"dict"`
	StopToken string `yaml:"stoptoken"`
	Store     string `yaml:"store"`
}

func init_engine(c *cli.Context) {
	runtimepath := path.Dir(os.Args[0])
	configpath := c.GlobalString("config")
	fmt.Println(configpath)
	if !path.IsAbs(configpath) {
		configpath = path.Join(runtimepath, configpath)
	}
	content, err := ioutil.ReadFile(configpath)
	fmt.Println(string(content))
	if err != nil {
		logrus.Fatal(err)
	}
	err = yaml.Unmarshal(content, &conf)
	if err != nil {
		logrus.Fatal(err)
	}

	if conf.Dict == "" {
		conf.Dict = path.Join(runtimepath, "dict", "dictionary.txt")
	} else if !path.IsAbs(conf.Dict) {
		conf.Dict = path.Join(runtimepath, conf.Dict)
	}
	if conf.StopToken == "" {
		conf.Dict = path.Join(runtimepath, "dict", "stop_tokens.txt")
	} else if !path.IsAbs(conf.StopToken) {
		conf.StopToken = path.Join(runtimepath, conf.StopToken)
	}
	if conf.Store == "" {
		conf.Store = path.Join(runtimepath, "store")
	} else if !path.IsAbs(conf.Store) {
		conf.Store = path.Join(runtimepath, conf.Store)
	}

	e.Init(types.EngineInitOptions{
		StopTokenFile:           conf.StopToken,
		SegmenterDictionaries:   conf.Dict,
		UsePersistentStorage:    true,
		PersistentStorageShards: 8,
		PersistentStorageFolder: conf.Store,
	})
}

func close_engine() {
	logrus.Info("engine close")
	e.Close()
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
		e.IndexDocument(uint64(time.Now().Unix()), types.DocumentIndexData{
			Content: title + content,
		}, true)
		logrus.Info("indexed %v", title)
		logrus.Info("index size: %v", e.NumDocumentsIndexed())
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

	resp := e.Search(types.SearchRequest{
		Text: keyword,
	})
	logrus.Info(e.NumDocumentsIndexed())
	if resp.NumDocs == 0 {
		logrus.Info("未找到结果")
	} else {

		logrus.Infof("找到 %v 条结果", resp.NumDocs)
		for _, doc := range resp.Docs {
			logrus.Infof("[%v]", doc.DocId)
		}
	}
	return nil
}

func web(c *cli.Context) error {
	init_engine(c)
	fmt.Println("web")
	return nil
}
