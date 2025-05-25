# Transcoder

Running this project locally requeires a few steps:

1. Install Docker
2. Have a `video.mp4` file inside `/assets` directory in the root of the project (will make it dynamic later)
3. Run the following command to build and run the Docker container:

```bash
# Build the Docker image
docker build -t transcoder .
```

```bash
# Run the Docker container
docker run transcoder
```
