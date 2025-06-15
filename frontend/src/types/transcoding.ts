export interface ProgressDetails {
	resolution: string;
	frame: string;
	timestamp: number;
}

export interface TranscodingStatusUpdate {
	type: "started" | "progress" | "completed" | "failed" | "cancelled";
	message: string;
	progress?: number;
	data?: ProgressDetails;
	downloadUrl?: string;
	timestamp: number;
}

export interface TranscodeResponse {
	/**
	 * A confirmation message from the server.
	 * @example "Transcoding of my-video.mp4 started successfully."
	 */
	message: string;

	/**
	 * The unique identifier for this transcoding job.
	 * This ID is used to get status updates and cancel the job.
	 * @example "a1b2c3d4-e5f6-7890-1234-567890abcdef"
	 */
	taskId: string;

	/**
	 * The relative URL path for the Server-Sent Events (SSE) stream
	 * to get real-time status updates.
	 * @example "/transcode/status/a1b2c3d4-e5f6-7890-1234-567890abcdef"
	 */
	statusStreamUrl: string;
}

export interface CancelTranscodeResponse {
	/**
	 * A message confirming the result of the cancellation request.
	 * @example "Cancellation request for task a1b2c3d4-e5f6-7890-1234-567890abcdef received."
	 */
	message: string;

	/**
	 * The ID of the task that was requested to be cancelled.
	 */
	taskId: string;
}

export interface TranscodingJob {
	taskId: string;
	filename: string;
	status: "uploading" | "transcoding" | "completed" | "failed" | "cancelled";
	progress: number;
	resolutions: ResolutionProgress[];
	downloadUrl?: string;
	createdAt: number;
	message: string;
}

export interface ResolutionProgress {
	resolution: string;
	progress: number;
	status: "pending" | "transcoding" | "completed" | "failed";
	frame?: string;
}
