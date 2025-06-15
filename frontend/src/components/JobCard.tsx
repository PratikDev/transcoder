import { Badge } from "@/components/ui/badge";
import { Button } from "@/components/ui/button";
import { Card, CardContent, CardHeader } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";
import { type TranscodingJob } from "@/types/transcoding";
import {
	AlertCircle,
	CheckCircle,
	Clock,
	Download,
	Video,
	X,
	XCircle,
} from "lucide-react";
import React from "react";

interface JobCardProps {
	job: TranscodingJob;
	onCancel: (taskId: string) => void;
	onDownload: (url: string, filename: string) => void;
}

const JobCard: React.FC<JobCardProps> = ({ job, onCancel, onDownload }) => {
	const getStatusIcon = () => {
		switch (job.status) {
			case "uploading":
				return <Clock className="h-4 w-4" />;
			case "transcoding":
				return <Clock className="h-4 w-4 animate-spin" />;
			case "completed":
				return <CheckCircle className="h-4 w-4" />;
			case "failed":
				return <XCircle className="h-4 w-4" />;
			case "cancelled":
				return <AlertCircle className="h-4 w-4" />;
			default:
				return <Clock className="h-4 w-4" />;
		}
	};

	const getStatusColor = () => {
		switch (job.status) {
			case "uploading":
				return "bg-blue-500";
			case "transcoding":
				return "bg-yellow-500";
			case "completed":
				return "bg-green-500";
			case "failed":
				return "bg-red-500";
			case "cancelled":
				return "bg-gray-500";
			default:
				return "bg-gray-500";
		}
	};

	const formatTime = (timestamp: number) => {
		return new Date(timestamp).toLocaleTimeString();
	};

	return (
		<Card className="overflow-hidden">
			<CardHeader className="pb-3">
				<div className="flex items-center justify-between">
					<div className="flex items-center space-x-3">
						<Video className="h-6 w-6 text-primary" />

						<div>
							<h3 className="font-semibold truncate">{job.filename}</h3>
							<p className="text-sm text-muted-foreground">
								Started at {formatTime(job.createdAt)}
							</p>
						</div>
					</div>

					<div className="flex items-center space-x-2">
						<Badge
							variant="outline"
							className="capitalize"
						>
							<div className="flex items-center space-x-1">
								{getStatusIcon()}
								<span>{job.status}</span>
							</div>
						</Badge>

						{job.status === "transcoding" && (
							<Button
								variant="outline"
								size="sm"
								onClick={() => onCancel(job.taskId)}
								className="h-8 w-8 p-0"
							>
								<X className="h-4 w-4" />
							</Button>
						)}
					</div>
				</div>

				<p className="text-xs text-muted-foreground mt-1 bg-accent py-0.5 px-2 rounded-full">
					<code>{job.taskId}</code>
				</p>
			</CardHeader>

			<CardContent className="space-y-4">
				{job.status !== "completed" && (
					<div className="space-y-2">
						<div className="flex justify-between text-sm">
							<span>Overall Progress</span>
							<span>{Math.round(job.progress)}%</span>
						</div>
						<Progress
							value={job.progress}
							className="h-2"
						/>
					</div>
				)}

				{job.resolutions.length > 0 && (
					<div className="space-y-3">
						<h4 className="text-sm font-medium">Resolution Progress</h4>
						<div className="grid gap-2">
							{job.resolutions.map((resolution) => (
								<div
									key={resolution.resolution}
									className="space-y-1"
								>
									<div className="flex items-center justify-between text-sm">
										<span className="font-mono">{resolution.resolution}</span>
										<div className="flex items-center space-x-2">
											<span className="capitalize text-xs">
												{resolution.status}
											</span>
											<span>{Math.round(resolution.progress)}%</span>
										</div>
									</div>
									<div className="flex items-center space-x-2">
										<Progress
											value={resolution.progress}
											className="h-1 flex-1"
										/>
										<div
											className={`w-2 h-2 rounded-full ${
												resolution.status === "completed"
													? "bg-green-500"
													: resolution.status === "transcoding"
													? "bg-yellow-500"
													: resolution.status === "failed"
													? "bg-red-500"
													: "bg-gray-300"
											}`}
										/>
									</div>
									{resolution.frame && (
										<p className="text-xs text-muted-foreground pl-2">
											Frame: {resolution.frame}
										</p>
									)}
								</div>
							))}
						</div>
					</div>
				)}

				{job.message && (
					<div className="text-sm text-muted-foreground bg-accent p-3 rounded-md">
						{job.message}
					</div>
				)}

				{job.status === "completed" && job.downloadUrl && (
					<Button
						onClick={() => onDownload(job.downloadUrl!, job.filename)}
						className="w-full"
						variant="default"
					>
						<Download className="h-4 w-4 mr-2" />
						Download HLS Package
					</Button>
				)}
			</CardContent>
		</Card>
	);
};

export default JobCard;
