include .env

CLIENT ?= podman
# RELEASE_VERSION is overridden in CI as the git tag matching format v[0-9]+.[0-9]+.[0-9]+
RELEASE_VERSION ?= dev

all:

ifdef REGISTRY_AUTH_FILE
AUTHFILE_SECRET = --secret id=authfile,src=$(REGISTRY_AUTH_FILE)
else
AUTHFILE_SECRET =
endif

build-golang-executable:
	$(CLIENT) run --rm -v ${PWD}:/usr/src:Z -w /usr/src docker.io/golang:1.25.8 go build -v \
	-ldflags "-X 'github.com/quay/mirror-registry/cmd.releaseVersion=${RELEASE_VERSION}' -X 'github.com/quay/mirror-registry/cmd.eeImage=${EE_IMAGE}' -X 'github.com/quay/mirror-registry/cmd.pauseImage=${PAUSE_IMAGE}' -X 'github.com/quay/mirror-registry/cmd.quayImage=${QUAY_IMAGE}' -X 'github.com/quay/mirror-registry/cmd.redisImage=${REDIS_IMAGE}' -X 'github.com/quay/mirror-registry/cmd.sqliteImage=${SQLITE_IMAGE}'" \
	-o mirror-registry;

build-online-zip: 
	$(CLIENT) build \
		-t mirror-registry-online:${RELEASE_VERSION} \
		--build-arg RELEASE_VERSION=${RELEASE_VERSION} \
		--build-arg QUAY_IMAGE=${QUAY_IMAGE} \
		--build-arg EE_IMAGE=${EE_IMAGE} \
		--build-arg EE_BASE_IMAGE=${EE_BASE_IMAGE} \
		--build-arg EE_BUILDER_IMAGE=${EE_BUILDER_IMAGE} \
		--build-arg REDIS_IMAGE=${REDIS_IMAGE} \
		--build-arg PAUSE_IMAGE=${PAUSE_IMAGE} \
		--build-arg SQLITE_IMAGE=${SQLITE_IMAGE} \
		--file Dockerfile.online . 
	$(CLIENT) run --name mirror-registry-online-${RELEASE_VERSION} mirror-registry-online:${RELEASE_VERSION}
	$(CLIENT) cp mirror-registry-online-${RELEASE_VERSION}:/mirror-registry.tar.gz .
	$(CLIENT) rm mirror-registry-online-${RELEASE_VERSION}

build-seed-images:
	$(CLIENT) build \
		--target seed-images-builder \
		-t mirror-registry-seed-images:${RELEASE_VERSION} \
		$(AUTHFILE_SECRET) \
		--file Dockerfile .
	$(CLIENT) create --name mirror-registry-seed-images-${RELEASE_VERSION} mirror-registry-seed-images:${RELEASE_VERSION}
	$(CLIENT) cp mirror-registry-seed-images-${RELEASE_VERSION}:/seed-images.tar .
	$(CLIENT) rm mirror-registry-seed-images-${RELEASE_VERSION}

build-offline-zip:
	$(CLIENT) build \
		-t mirror-registry-offline:${RELEASE_VERSION} \
		--build-arg RELEASE_VERSION=${RELEASE_VERSION} \
		--build-arg QUAY_IMAGE=${QUAY_IMAGE} \
		--build-arg EE_IMAGE=${EE_IMAGE} \
		--build-arg EE_BASE_IMAGE=${EE_BASE_IMAGE} \
		--build-arg EE_BUILDER_IMAGE=${EE_BUILDER_IMAGE} \
		--build-arg REDIS_IMAGE=${REDIS_IMAGE} \
		--build-arg PAUSE_IMAGE=${PAUSE_IMAGE} \
		--build-arg SQLITE_IMAGE=${SQLITE_IMAGE} \
		$(AUTHFILE_SECRET) \
		--file Dockerfile .
	$(CLIENT) run --name mirror-registry-offline-${RELEASE_VERSION} mirror-registry-offline:${RELEASE_VERSION}
	$(CLIENT) cp mirror-registry-offline-${RELEASE_VERSION}:/mirror-registry.tar.gz .
	$(CLIENT) rm mirror-registry-offline-${RELEASE_VERSION}

clean:
	rm -rf mirror-registry* image-archive.tar seed-images.tar
