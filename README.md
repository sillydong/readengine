ReadEngine
===

ReadEngine can index main contents of the url and index into search engine. You can search later by keyword.

ReadEngine used the `goreadability` from [https://github.com/philipjkim/goreadability](https://github.com/philipjkim/goreadability) and copied the main code into this project as it used `goquery` from its vendor which is an older version. Also I changed the fastimage from [https://github.com/philipjkim/fastimage](https://github.com/philipjkim/fastimage) to [https://github.com/sillydong/fastimage](https://github.com/sillydong/fastimage) because my fastimage supports more image formats.

## How To Install

ReadEngine need `CGO_ENABLE=1` because it used the `gojieba` library to parse Chinese.

- `make` will make the bin in the code path.
- `make install` will make and install to your `$GOBIN` path with config.yaml and dict.
- `make clean` will clean the bin in the code path

## How To Use

- Index
	```
	readengine url "https://geeks.uniplaces.com/building-a-worker-pool-in-golang-1e6c0fdfd78c"
	```
- Search
	```
	readengine search "go"
	```
- Rebuild
	```
	readengine rebuild
	```
- History
	```
	readengine history
	```

### TODO

- maybe index file contents?
- maybe a web page for search and read?
- maybe packup webpage for later read to avoid the source page be deleted?
- maybe a better search engine?
