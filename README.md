# Transcoder

Running this project locally requeires a few steps:

1. Install Docker
2. Have a `video.mp4` file inside `/assets` directory in the root of the project (will make it dynamic later)
3. Run the following command to build and run the Docker container:

```bash
docker build -t transcoder .
```

```bash
docker run transcoder
```

## Next Steps

- [x] Wrap everything into a RESTful API.
- [x] Accept source file in the request body.
- [x] Detect the resolution of the source video and automatically transcode to all lower resolutions in descending order.
- [ ] Currently the transcoding process is running in the same thread as the request handler, which blocks the server. This should run in a separate thread or process. User should be able to check the status of the transcoding process with a GET request.
- [ ] Add security measures to prevent abuse.
