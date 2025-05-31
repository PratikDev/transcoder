IMAGE_NAME=transcoder
CONTAINER_NAME=transcoder
SERVER_PORT=3000
BASE_COMMAND=sudo docker

.PHONY: build run go clean

build:
	$(BASE_COMMAND) build -t $(IMAGE_NAME) .

run:
	$(BASE_COMMAND) run -p $(SERVER_PORT):$(SERVER_PORT) --name $(CONTAINER_NAME) -it $(IMAGE_NAME)

go:
	@$(MAKE) build && clear && $(MAKE) run

clean:
	-$(BASE_COMMAND) rm -f $(CONTAINER_NAME)
	-$(BASE_COMMAND) rmi -f $(IMAGE_NAME)