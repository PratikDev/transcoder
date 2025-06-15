import { Upload, Video, X } from "lucide-react";
import React, { useCallback, useState } from "react";
import { useDropzone } from "react-dropzone";

import { Button } from "@/components/ui/button";
import { Card } from "@/components/ui/card";
import { Progress } from "@/components/ui/progress";

interface VideoUploadProps {
	onUpload: (file: File) => void;
	isUploading: boolean;
	uploadProgress: number;
}

const VideoUpload: React.FC<VideoUploadProps> = ({
	onUpload,
	isUploading,
	uploadProgress,
}) => {
	const [selectedFile, setSelectedFile] = useState<File | null>(null);

	const onDrop = useCallback((acceptedFiles: File[]) => {
		if (acceptedFiles.length > 0) {
			setSelectedFile(acceptedFiles[0]);
		}
	}, []);

	const { getRootProps, getInputProps, isDragActive, isDragReject } =
		useDropzone({
			onDrop,
			accept: {
				"video/*": [".mp4", ".mov", ".avi", ".mkv", ".webm"],
			},
			multiple: false,
			disabled: isUploading,
		});

	const handleUpload = () => {
		if (selectedFile) {
			onUpload(selectedFile);
		}
	};

	const clearFile = () => {
		setSelectedFile(null);
	};

	return (
		<Card className="p-8">
			<div className="space-y-6">
				<div className="text-center">
					<Video className="mx-auto h-12 w-12 text-primary mb-4" />
					<h2 className="text-2xl font-bold mb-2">Video Transcoding Service</h2>
					<p className="text-muted-foreground">
						Upload your video to generate HLS transcodes in multiple resolutions
					</p>
				</div>

				{!selectedFile ? (
					<div
						{...getRootProps()}
						className={`
              border-2 border-dashed rounded-lg p-12 text-center cursor-pointer transition-all duration-200
              ${
								isDragActive && !isDragReject
									? "border-primary bg-primary/5"
									: isDragReject
									? "border-destructive bg-destructive/5"
									: "border-border hover:border-primary/50 hover:bg-accent/50"
							}
              ${isUploading ? "opacity-50 cursor-not-allowed" : ""}
            `}
					>
						<input {...getInputProps()} />
						<Upload className="mx-auto h-8 w-8 text-muted-foreground mb-4" />
						{isDragActive ? (
							<p className="text-primary font-medium">
								Drop your video file here...
							</p>
						) : isDragReject ? (
							<p className="text-destructive font-medium">
								Please select a valid video file
							</p>
						) : (
							<div>
								<p className="font-medium mb-2">
									Drag & drop your video file here, or click to browse
								</p>

								<p className="text-sm text-muted-foreground">
									Supports MP4, MOV, AVI, MKV, WebM files
								</p>
							</div>
						)}
					</div>
				) : (
					<div className="space-y-4">
						<div className="flex items-center justify-between p-4 bg-accent rounded-lg">
							<div className="flex items-center space-x-3">
								<Video className="h-6 w-6 text-primary" />

								<div>
									<p className="font-medium">{selectedFile.name}</p>
									<p className="text-sm text-muted-foreground">
										{(selectedFile.size / 1024 / 1024).toFixed(2)} MB
									</p>
								</div>
							</div>

							{!isUploading && (
								<Button
									variant="ghost"
									size="sm"
									onClick={clearFile}
									className="h-8 w-8 p-0"
								>
									<X className="h-4 w-4" />
								</Button>
							)}
						</div>

						{isUploading && (
							<div className="space-y-2">
								<div className="flex justify-between text-sm">
									<span>Uploading...</span>
									<span>{uploadProgress}%</span>
								</div>
								<Progress
									value={uploadProgress}
									className="h-2"
								/>
							</div>
						)}

						<Button
							onClick={handleUpload}
							disabled={isUploading}
							className="w-full"
							size="lg"
						>
							{isUploading ? "Uploading..." : "Start Transcoding"}
						</Button>
					</div>
				)}
			</div>
		</Card>
	);
};

export default VideoUpload;
