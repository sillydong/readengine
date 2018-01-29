GOCMD=go
GOBUILD=$(GOCMD) build
GOINSTALL=$(GOCMD) install
GOCLEAN=$(GOCMD) clean
BINNAME=readengine
GOARCH=amd64
GOOS=darwin

build:
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOBUILD) -o $(BINNAME) -v ./

install:
	CGO_ENABLED=1 GOOS=$(GOOS) GOARCH=$(GOARCH) $(GOINSTALL) -v ./

conf:
	cp -R dict_jieba $(GOPATH)/bin
	cp -R config.yaml $(GOPATH)/bin

clean:
	rm -f $(BINNAME)

fmt:
	go fmt ./
