import type { Resolutions } from "@/constant/transcoder";
import type { LogProgressStatus } from "./logger";

export type Progress = {
	percent?: number;
	file: string;
	status?: LogProgressStatus;
};

export type Step = {
	index: number;
	process: string;
	file: string;
	percent?: number;
	status?: LogProgressStatus;
};

export type TranscoderSource = {
	file: string;
	filename: string;
	extname: string;
};

export type TranscoderOptions = {
	source: TranscoderSource;
	resolutions: Resolutions[];
	output: string;
	includeWebp: boolean;
};

export type TranscoderPlaylist = {
	resolution: { width: number; height: number };
	bitrate: number;
	playlistFilename: string;
	playlistPathFromMain: string;
	playlistPath: string;
};
