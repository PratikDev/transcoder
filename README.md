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
- [x] Implement SSE to stream the transcoding progress to the client.
- [ ] Need to disconnect the SSE stream when the transcoding is completed.
- [ ] Test with multiple subscribers to the same transcoding job.
- [ ] Add security measures to prevent abuse.
