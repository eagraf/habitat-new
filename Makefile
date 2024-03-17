
MIN_MAKE_VERSION	=	4.0.0

ifneq ($(MIN_MAKE_VERSION),$(firstword $(sort $(MAKE_VERSION) $(MIN_MAKE_VERSION))))
$(error you must have a version of GNU make newer than v$(MIN_MAKE_VERSION) installed)
endif

# If TOPDIR isn't already defined, let's go with a default
ifeq ($(origin TOPDIR), undefined)
TOPDIR			:=	$(realpath $(patsubst %/,%, $(dir $(lastword $(MAKEFILE_LIST)))))
endif

# Directories inside the dev docker container
DOCKER_WORKDIR = /go/src/github.com/eagraf/habitat-new

# Set up critical Habitat environment variables
DEV_HABITAT_PATH = $(TOPDIR)/.habitat
CERT_DIR = $(DEV_HABITAT_PATH)/certificates

GOBIN ?= $$(go env GOPATH)/bin

build: $(TOPDIR)/build/amd64-linux/habitat $(TOPDIR)/build/amd64-darwin/habitat

test::
	go test ./... -timeout 1s

test-coverage:
	go test ./... -coverprofile=coverage.out -coverpkg=./... -timeout 1s
	${GOBIN}/go-test-coverage --config=./.testcoverage.yml || true
	go tool cover -html=coverage.out

lint::
# To install: https://golangci-lint.run/usage/install/#local-installation
	CGO_ENABLED=0 golangci-lint run ./...

install:: $(DEV_HABITAT_PATH)/habitat.yml $(CERT_DIR)/dev_node_cert.pem $(CERT_DIR)/dev_root_user_cert.pem

docker-build:
	docker build -t habitat_node -f ./build/node.dev.Dockerfile .

run-dev:
	docker run -p 3000:3000 -p 3001:3001 -p 4000:4000 \
		-v $(TOPDIR)/core:$(DOCKER_WORKDIR)/core\
		-v $(TOPDIR)/cmd:$(DOCKER_WORKDIR)/cmd \
		-v $(TOPDIR)/internal:$(DOCKER_WORKDIR)/internal \
		-v $(TOPDIR)/pkg:$(DOCKER_WORKDIR)/pkg \
		-v $(TOPDIR)/.habitat:/.habitat \
		-v /var/run/docker.sock:/var/run/docker.sock \
		-e HABITAT_PATH=/.habitat \
		habitat_node

clear-volumes:
	docker container rm -f habitat_node || true
	docker volume prune -f
	rm -rf $(DEV_HABITAT_PATH)/hdb

run-dev-fresh: clear-volumes run-dev

$(DEV_HABITAT_PATH):
	mkdir -p $(DEV_HABITAT_PATH)

$(CERT_DIR): $(DEV_HABITAT_PATH)
	mkdir -p $(CERT_DIR)

$(DEV_HABITAT_PATH)/habitat.yml: $(DEV_HABITAT_PATH)
	cp $(TOPDIR)/config/habitat.dev.yml $(DEV_HABITAT_PATH)/habitat.yml

$(CERT_DIR)/dev_node_cert.pem: $(CERT_DIR)
	@echo "Generating dev node certificate"
	openssl req -newkey rsa:2048 \
		-new -nodes -x509 \
		-out $(CERT_DIR)/dev_node_cert.pem \
		-keyout $(CERT_DIR)/dev_node_key.pem \
		-subj "/C=US/ST=California/L=Mountain View/O=Habitat/CN=dev_node"

$(CERT_DIR)/dev_root_user_cert.pem: $(CERT_DIR)
	@echo "Generating dev root user certificate"
	openssl req -newkey rsa:2048 \
		-new -nodes -x509 \
		-out $(CERT_DIR)/dev_root_user_cert.pem \
		-keyout $(CERT_DIR)/dev_root_user_key.pem \
		-subj "/C=US/ST=California/L=Mountain View/O=Habitat/CN=root"

$(TOPDIR)/build: $(TOPDIR)
	mkdir -p $(TOPDIR)/build

$(TOPDIR)/build/amd64-linux/habitat: $(TOPDIR)/build
	GOARCH=amd64 GOOS=linux go build -o $(TOPDIR)/build/amd64-linux/habitat $(TOPDIR)/cmd/node/main.go

$(TOPDIR)/build/amd64-darwin/habitat: $(TOPDIR)/build
	GOARCH=amd64 GOOS=darwin go build -o $(TOPDIR)/build/amd64-darwin/habitat $(TOPDIR)/cmd/node/main.go
