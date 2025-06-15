import JobCard from "@/components/JobCard";
import { Card, CardContent, CardHeader, CardTitle } from "@/components/ui/card";
import { Separator } from "@/components/ui/separator";
import VideoUpload from "@/components/VideoUpload";
import { useTranscoding } from "@/hooks/useTranscoding";

export default function Index() {
	const {
		jobs,
		isUploading,
		uploadProgress,
		startTranscoding,
		cancelJob,
		downloadFile,
	} = useTranscoding();

	const activeJobs = jobs.filter(
		(job) => job.status === "transcoding" || job.status === "uploading"
	);
	const completedJobs = jobs.filter((job) => job.status === "completed");
	const otherJobs = jobs.filter(
		(job) => job.status === "failed" || job.status === "cancelled"
	);

	return (
		<div className="min-h-screen bg-gradient-to-br from-slate-50 to-slate-100 p-4">
			<div className="max-w-6xl mx-auto space-y-8">
				{/* Header */}
				<div className="text-center py-8">
					<h1 className="text-4xl font-bold text-slate-900 mb-4">
						HLS Video Transcoding Service
					</h1>
					<p className="text-lg text-slate-600 max-w-2xl mx-auto">
						Upload your videos and get high-quality HLS transcodes in multiple
						resolutions with real-time progress tracking
					</p>
				</div>

				{/* Upload Section */}
				<VideoUpload
					onUpload={startTranscoding}
					isUploading={isUploading}
					uploadProgress={uploadProgress}
				/>

				{/* Jobs Dashboard */}
				{jobs.length > 0 && (
					<div className="space-y-6">
						<div className="flex items-center space-x-4">
							<h2 className="text-2xl font-bold">Transcoding Jobs</h2>
							<div className="flex space-x-2">
								{activeJobs.length > 0 && (
									<span className="px-3 py-1 bg-yellow-100 text-yellow-800 text-sm rounded-full">
										{activeJobs.length} Active
									</span>
								)}
								{completedJobs.length > 0 && (
									<span className="px-3 py-1 bg-green-100 text-green-800 text-sm rounded-full">
										{completedJobs.length} Completed
									</span>
								)}
							</div>
						</div>

						{/* Active Jobs */}
						{activeJobs.length > 0 && (
							<div className="space-y-4">
								<h3 className="text-lg font-semibold text-slate-700">
									Active Jobs
								</h3>
								<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
									{activeJobs.map((job) => (
										<JobCard
											key={job.taskId}
											job={job}
											onCancel={cancelJob}
											onDownload={downloadFile}
										/>
									))}
								</div>
							</div>
						)}

						{/* Completed Jobs */}
						{completedJobs.length > 0 && (
							<div className="space-y-4">
								<h3 className="text-lg font-semibold text-slate-700">
									Completed Jobs
								</h3>
								<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
									{completedJobs.map((job) => (
										<JobCard
											key={job.taskId}
											job={job}
											onCancel={cancelJob}
											onDownload={downloadFile}
										/>
									))}
								</div>
							</div>
						)}

						{/* Other Jobs (Failed/Cancelled) */}
						{otherJobs.length > 0 && (
							<div className="space-y-4">
								<h3 className="text-lg font-semibold text-slate-700">
									Other Jobs
								</h3>
								<div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
									{otherJobs.map((job) => (
										<JobCard
											key={job.taskId}
											job={job}
											onCancel={cancelJob}
											onDownload={downloadFile}
										/>
									))}
								</div>
							</div>
						)}
					</div>
				)}

				{/* Getting Started Guide */}
				{jobs.length === 0 && (
					<Card className="max-w-2xl mx-auto">
						<CardHeader>
							<CardTitle>Getting Started</CardTitle>
						</CardHeader>
						<CardContent className="space-y-4">
							<div className="space-y-3">
								<div className="flex items-start space-x-3">
									<div className="w-6 h-6 bg-primary text-primary-foreground rounded-full flex items-center justify-center text-sm font-bold">
										1
									</div>
									<div>
										<h4 className="font-medium">Upload Your Video</h4>
										<p className="text-sm text-muted-foreground">
											Drag and drop or click to select a video file (MP4, MOV,
											AVI, MKV, WebM)
										</p>
									</div>
								</div>
								<Separator />
								<div className="flex items-start space-x-3">
									<div className="w-6 h-6 bg-primary text-primary-foreground rounded-full flex items-center justify-center text-sm font-bold">
										2
									</div>
									<div>
										<h4 className="font-medium">Monitor Progress</h4>
										<p className="text-sm text-muted-foreground">
											Watch real-time progress updates for each resolution being
											transcoded
										</p>
									</div>
								</div>
								<Separator />
								<div className="flex items-start space-x-3">
									<div className="w-6 h-6 bg-primary text-primary-foreground rounded-full flex items-center justify-center text-sm font-bold">
										3
									</div>
									<div>
										<h4 className="font-medium">Download Results</h4>
										<p className="text-sm text-muted-foreground">
											Once completed, download your HLS package with all
											resolution variants
										</p>
									</div>
								</div>
							</div>
						</CardContent>
					</Card>
				)}
			</div>
		</div>
	);
}
