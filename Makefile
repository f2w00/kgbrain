APP_NAME = kgbrain
REGISTRY = 10.15.22.234:5005
IMAGE = $(REGISTRY)/$(APP_NAME)
TAG = latest
SHA = $(shell git rev-parse --short HEAD)
DATA_DIR = ./data

.PHONY: all build test docker-build docker-push docker-run docker-clean

all: test docker-build

build:
	CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -o $(APP_NAME) ./cmd/server

test:
	go test ./tests/... -count=1

docker-build:
	docker build -t $(IMAGE):$(TAG) -t $(IMAGE):$(SHA) .
	@echo "Built: $(IMAGE):$(TAG) $(IMAGE):$(SHA)"

docker-push:
	docker push $(IMAGE):$(TAG)
	# docker push $(IMAGE):$(SHA)
	@echo "Pushed: $(IMAGE):$(TAG) $(IMAGE):$(SHA)"

docker-run:
	@if [ $$(docker ps -aq -f name=^/$(APP_NAME)$$) ]; then docker rm -f $(APP_NAME); fi
	docker run -d \
		--name $(APP_NAME) \
		-p 8848:8848 \
		-v $(PWD)/data:/app/data \
		$(IMAGE):$(TAG)

docker-clean:
	docker rmi $(IMAGE):$(TAG) $(IMAGE):$(SHA) 2>/dev/null || true
