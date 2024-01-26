
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
CERT_DIR = /dev_certificates

test::
	go test ./...

lint::
	CGO_ENABLED=0 golangci-lint run --skip-dirs '(^|/)virtctl($$|/)' -D errcheck ./...

docker-build:
	docker build -t habitat_node -f ./build/node.dev.Dockerfile .

run-dev:
	docker run -p 3000:3000 -p 3001:3001 -p 4000:4000 \
		-v $(TOPDIR)/api:$(DOCKER_WORKDIR)/api\
		-v $(TOPDIR)/cmd:$(DOCKER_WORKDIR)/cmd \
		-v $(TOPDIR)/internal:$(DOCKER_WORKDIR)/internal \
		-v $(TOPDIR)/pkg:$(DOCKER_WORKDIR)/pkg \
		-v $(HOME)/.ssh/habitat_dev:$(CERT_DIR) \
		habitat_node
