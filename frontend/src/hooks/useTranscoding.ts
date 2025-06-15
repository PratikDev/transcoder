import axios from "axios";
import { useCallback, useEffect, useState } from "react";
import { toast } from "sonner";

import type {
	CancelTranscodeResponse,
	TranscodeResponse,
	TranscodingJob,
} from "@/types/transcoding";

export const useTranscoding = () => {
	const [jobs, setJobs] = useState<TranscodingJob[]>([]);
	const [isUploading, setIsUploading] = useState(false);
	const [uploadProgress, setUploadProgress] = useState(0);
	const [eventSources, setEventSources] = useState<Map<string, EventSource>>(
		new Map()
	);

	const uploadFile = useCallback(
		async (file: File): Promise<TranscodeResponse> => {
			setIsUploading(true);
			setUploadProgress(0);

			const formData = new FormData();
			formData.append("video", file);

			try {
				const response = await axios.post<TranscodeResponse>(
					"http://localhost:3000/transcode",
					formData,
					{
						headers: {
							"Content-Type": "multipart/form-data",
						},
						onUploadProgress: (progressEvent) => {
							if (progressEvent.total) {
								const percent = Math.round(
									(progressEvent.loaded * 100) / progressEvent.total
								);
								setUploadProgress(percent);
							}
						},
					}
				);

				setIsUploading(false);
				setUploadProgress(0);

				return response.data;
			} catch (error: any) {
				setIsUploading(false);
				setUploadProgress(0);
				throw new Error(
					error?.response?.data?.message || "Upload failed. Please try again."
				);
			}
		},
		[]
	);

	// Start transcoding job
	const startTranscoding = useCallback(
		async (file: File) => {
			try {
				const response = await uploadFile(file);

				const newJob: TranscodingJob = {
					taskId: response.taskId,
					filename: file.name,
					status: "transcoding",
					progress: 0,
					resolutions: [
						{ resolution: "1080p", progress: 0, status: "pending" },
						{ resolution: "720p", progress: 0, status: "pending" },
						{ resolution: "480p", progress: 0, status: "pending" },
					],
					createdAt: Date.now(),
					message: response.message,
				};

				setJobs((prev) => [newJob, ...prev]);

				// Start SSE connection for this job
				connectToStatusStream(response.taskId, response.statusStreamUrl);

				toast("Transcoding Started", {
					description: response.message,
				});
			} catch (error) {
				console.error("Failed to start transcoding:", error);
				toast.error("Failed to start transcoding.", {
					description: "Please try again.",
				});
			}
		},
		[uploadFile]
	);

	// Connect to SSE stream for job updates
	const connectToStatusStream = useCallback(
		(taskId: string, streamUrl: string) => {
			// For demo purposes, simulate SSE updates
			simulateTranscodingProgress(taskId);
		},
		[]
	);

	// Simulate transcoding progress (replace with actual SSE connection)
	const simulateTranscodingProgress = useCallback((taskId: string) => {
		let progress = 0;
		let currentResolutionIndex = 0;
		const resolutions = ["1080p", "720p", "480p"];

		const interval = setInterval(() => {
			progress += Math.random() * 15;

			if (progress >= 100) {
				progress = 100;
				clearInterval(interval);

				// Mark job as completed
				setJobs((prev) =>
					prev.map((job) =>
						job.taskId === taskId
							? {
									...job,
									status: "completed",
									progress: 100,
									downloadUrl: `https://example.com/download/${taskId}`,
									resolutions: job.resolutions.map((r) => ({
										...r,
										progress: 100,
										status: "completed" as const,
									})),
									message: "Transcoding completed successfully!",
							  }
							: job
					)
				);

				toast.success("Transcoding Complete", {
					description: "Your video has been successfully transcoded!",
				});
				return;
			}

			// Update current resolution progress
			const currentResolution = resolutions[currentResolutionIndex];
			const resolutionProgress = Math.min(
				(progress - currentResolutionIndex * 33.33) * 3,
				100
			);

			if (
				resolutionProgress >= 100 &&
				currentResolutionIndex < resolutions.length - 1
			) {
				currentResolutionIndex++;
			}

			setJobs((prev) =>
				prev.map((job) =>
					job.taskId === taskId
						? {
								...job,
								progress,
								resolutions: job.resolutions.map((r, index) => ({
									...r,
									progress:
										index < currentResolutionIndex
											? 100
											: index === currentResolutionIndex
											? resolutionProgress
											: 0,
									status:
										index < currentResolutionIndex
											? "completed"
											: index === currentResolutionIndex
											? "transcoding"
											: "pending",
									frame:
										index === currentResolutionIndex
											? `${Math.floor(resolutionProgress * 10)}`
											: r.frame,
								})),
						  }
						: job
				)
			);
		}, 500);
	}, []);

	// Cancel transcoding job
	const cancelJob = useCallback(
		async (taskId: string) => {
			try {
				// Simulate API call to cancel job
				const mockResponse: CancelTranscodeResponse = {
					message: `Cancellation request for task ${taskId} received.`,
					taskId,
				};

				setJobs((prev) =>
					prev.map((job) =>
						job.taskId === taskId
							? {
									...job,
									status: "cancelled",
									message: "Job cancelled by user",
							  }
							: job
					)
				);

				// Close SSE connection
				const eventSource = eventSources.get(taskId);
				if (eventSource) {
					eventSource.close();
					setEventSources((prev) => {
						const newMap = new Map(prev);
						newMap.delete(taskId);
						return newMap;
					});
				}

				toast("Job Cancelled", {
					description: mockResponse.message,
				});
			} catch (error) {
				console.error("Failed to cancel job:", error);
				toast.error("Failed to cancel job.", {
					description: "Please try again.",
				});
			}
		},
		[eventSources]
	);

	// Download transcoded file
	const downloadFile = useCallback((url: string, filename: string) => {
		// Simulate download
		const link = document.createElement("a");
		link.href = url;
		link.download = `${filename}-hls-package.zip`;
		document.body.appendChild(link);
		link.click();
		document.body.removeChild(link);

		toast("Download Started", {
			description: `Downloading HLS package for ${filename}`,
		});
	}, []);

	// Cleanup SSE connections on unmount
	useEffect(() => {
		return () => {
			eventSources.forEach((eventSource) => eventSource.close());
		};
	}, [eventSources]);

	return {
		jobs,
		isUploading,
		uploadProgress,
		startTranscoding,
		cancelJob,
		downloadFile,
	};
};
