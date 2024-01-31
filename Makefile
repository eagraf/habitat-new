
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

test::
	go test ./...

lint::
	CGO_ENABLED=0 golangci-lint run --skip-dirs '(^|/)virtctl($$|/)' -D errcheck ./...

install:: $(DEV_HABITAT_PATH)/habitat.yml $(CERT_DIR)/dev_node_cert.pem $(CERT_DIR)/dev_root_user_cert.pem

docker-build:
	docker build -t habitat_node -f ./build/node.dev.Dockerfile .

run-dev:
	docker run -p 3000:3000 -p 3001:3001 -p 4000:4000 \
		-v $(TOPDIR)/api:$(DOCKER_WORKDIR)/api\
		-v $(TOPDIR)/cmd:$(DOCKER_WORKDIR)/cmd \
		-v $(TOPDIR)/internal:$(DOCKER_WORKDIR)/internal \
		-v $(TOPDIR)/pkg:$(DOCKER_WORKDIR)/pkg \
		-v $(TOPDIR)/.habitat:/.habitat \
		-e HABITAT_PATH=/.habitat \
		habitat_node

 clear-volumes:
	docker container rm -f habitat_node || true
	docker volume prune -f

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
