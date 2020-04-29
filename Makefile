 # Go parameters
GOCMD=GO111MODULE=on go
VERSION=$($CI_BUILD_TAG)
BUILD=`date +%FT%T%z`
LDFLAGS=-ldflags "-w -s  -X main.Version=${VERSION} -X main.BuildDate=${BUILD}"
GOBUILD=CGO_ENABLED=0 GOOS=linux GOARCH=amd64 $(GOCMD) build -trimpath ${LDFLAGS}
GOCLEAN=$(GOCMD) clean
GOTEST=$(GOCMD) test
GOGET=$(GOCMD) get
BINARY_NAME=vcd-csi
BINARY_UNIX=$(BINARY_NAME)_unix
MAIN_DIR=github.com/f41gh7/vcd-csi/cmd/provisioner
TEST_ARGS= -covermode=atomic -coverprofile=coverage.txt -v

all: test build

report: 
	$(GOCMD) tool cover -html=coverage.txt
build:
	$(GOBUILD)  -o $(BINARY_NAME) -v $(MAIN_DIR)
test:
	echo 'mode: atomic' > coverage.txt  && \
	$(GOTEST) $(TEST_ARGS) github.com/f41gh7/vcd-csi/internal/driver/
	$(GOCMD) tool cover -func coverage.txt  | grep total

.PHONY:
mock:
	echo "generating mocks"
	mockgen -source pkg/vcd-client/client.go -destination mock/vcdclient/client.go
	mockgen -source internal/mount/mounter.go -destination mock/mounter/mount.go
clean:
	$(GOCLEAN)
	rm -f $(BINARY_NAME)
	rm -f $(BINARY_UNIX)

docker:
	$(GOBUILD)  -o $(BINARY_NAME) -v $(MAIN_DIR)
	docker build -t quay.io/f41gh7/vcd-csi:develop -f cmd/provisioner/Dockerfile .
	docker push quay.io/f41gh7/vcd-csi:develop
run:
	$(GOBUILD) -o $(BINARY_NAME) -v $(MAIN_DIR)
	./$(BINARY_NAME)

