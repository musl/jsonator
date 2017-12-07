BIN := $(shell basename $(CURDIR))

DEPS := "github.com/gin-gonic/gin"
DEPS += "github.com/orcaman/concurrent-map"
DEPS += "github.com/newrelic/go-agent"
DEPS += "github.com/kelseyhightower/envconfig"

.PHONY: all clean commands test

all: clean test $(BIN)

clean:
	go clean .

clobber: clean
	rm -fr vendor

vendor:
	mkdir -p vendor
	for repo in $(DEPS); do git clone https://$$repo vendor/$$repo; done
	rm -fr vendor/*/*/*/.git
	
$(BIN):
	go build .

test: 
	go test .

docker: clean
	GOOS=linux GOARCH=amd64 go build .
	docker-compose up --build

