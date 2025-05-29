# Transcoder

This project is a simple video transcoder that uses FFmpeg to transcode videos into multiple resolutions. It is designed to be run in a Docker container.
It provides a RESTful API to accept video files and streams the transcoding progress using Server-Sent Events (SSE).

## Features

- [x] Accepts video files via a RESTful API.
- [x] Detects the resolution of the source video.
- [x] Automatically transcodes the video to all lower resolutions in descending order.
- [x] Streams the transcoding progress to the client using Server-Sent Events (SSE).
- [x] Disconnects the SSE stream when the transcoding is completed.
- [x] User shouldn't be able to connect to non-existent task IDs.
- [ ] Supports multiple subscribers to the same transcoding job.
- [ ] Endpoint to cancel a transcoding job.
- [ ] Way to retrieve the transcoded video files.
- [ ] Security measures to prevent abuse.

## API Endpoints

- `/transcode` (POST): Accepts a video file and starts the transcoding process. Returns a task ID.
- `/transcode/status/<task_id>` (GET): Streams the transcoding progress for the given task ID using Server-Sent Events (SSE).
- `/status` (GET): Returns the status of the server.

## Requirements

- Docker

## Run Locally

To run the transcoder locally, you need to have Docker installed. Follow these steps:

1. Clone the repository:

```bash
git clone https://github.com/pratikdev/transcoder.git && cd transcoder
```

2. Build the Docker image:

```bash
docker build -t transcoder .
```

3. Run the Docker container:

```bash
docker run -p 3000:3000 -it transcoder
```

This will start the transcoder service on port 3000.

4. Test the API:

```bash
curl -X POST -F "video=@/path/to/video.mp4" http://localhost:3000/transcode
```

5. Check the status of a transcoding job:

```bash
curl -N http://localhost:3000/transcode/status/<task_id>
```

`<task_id>` should be replaced with the actual task ID returned from the `/transcode` endpoint.
