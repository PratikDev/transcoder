IMAGE_NAME=transcoder
CONTAINER_NAME=transcoder
SERVER_PORT=3000
BASE_COMMAND=sudo docker

.PHONY: build run go enter clean transcode transcode-status transcode-cancel

build:
	$(BASE_COMMAND) build -t $(IMAGE_NAME) .

run:
	$(BASE_COMMAND) run -p $(SERVER_PORT):$(SERVER_PORT) --name $(CONTAINER_NAME) -it $(IMAGE_NAME)

go:
	@$(MAKE) build && clear && $(MAKE) run

transcode:
	@if [ -z "$(file)" ]; then \
		echo "Usage: make transcode file=/path/to/video.mp4"; \
		exit 1; \
	fi
	curl -X POST -F "video=@$(file)" http://localhost:$(SERVER_PORT)/transcode

transcode-status:
	@if [ -z "$(taskId)" ]; then \
		echo "Usage: make transcode-status taskId=<task_id>"; \
		exit 1; \
	fi
	curl -N http://localhost:$(SERVER_PORT)/transcode/status/$(taskId)

transcode-cancel:
	@if [ -z "$(taskId)" ]; then \
		echo "Usage: make transcode-cancel taskId=<task_id>"; \
		exit 1; \
	fi
	curl -X DELETE http://localhost:$(SERVER_PORT)/transcode/jobs/$(taskId)

enter:
	$(BASE_COMMAND) exec -it $(CONTAINER_NAME) /bin/sh

clean:
	-$(BASE_COMMAND) rm -f $(CONTAINER_NAME)
	-$(BASE_COMMAND) rmi -f $(IMAGE_NAME)