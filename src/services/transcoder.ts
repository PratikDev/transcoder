import ffmpeg, { FfmpegCommand } from "fluent-ffmpeg";
import { mkdir, writeFile } from "node:fs/promises";

import {
	ALLOWED_EXTENSION,
	Resolutions,
	RESOLUTIONS,
} from "@/constant/transcoder";
import type {
	TranscoderOptions,
	TranscoderPlaylist,
	TranscoderSource,
} from "@/types/transcoder";
import logger from "./logger";

export default class Transcoder {
	static resolutions = RESOLUTIONS;

	#command: FfmpegCommand | null = null;
	#source: TranscoderSource = {
		file: "",
		filename: "",
		extname: "",
	};
	#resolutions: Resolutions[] = [];
	#output: string;
	#includeWebp: boolean;

	constructor({ source, resolutions, output, includeWebp }: TranscoderOptions) {
		this.#source = source;
		this.#resolutions = resolutions;
		this.#output = output;
		this.#includeWebp = includeWebp;
	}

	static parseResolutions(arg: string) {
		const parsed: Resolutions[] = [];
		const args = arg.split(",");

		for (const value of Object.values(Resolutions)) {
			const resolution = args.find((a) => a == value);
			if (resolution) parsed.push(value as Resolutions);
		}

		return parsed;
	}

	static parseSourceFile(file: string): TranscoderSource | null {
		const filename = file.split("/").pop() as string;
		const extname = filename.split(".").pop() as string;

		if (!ALLOWED_EXTENSION.includes(extname)) {
			console.error(`unsupported file type, file will be skipped: ${file}`);
			return null;
		}

		return {
			file,
			filename,
			extname,
		};
	}

	static async detectVideoResolution(path: string): Promise<Resolutions> {
		return new Promise((resolve, reject) => {
			ffmpeg(path).ffprobe((err, data) => {
				if (err) {
					return reject(err);
				}

				const { width, height } =
					data.streams.find((stream) => stream.codec_type === "video") ?? {};

				if (!width || !height) {
					return reject(new Error("Could not detect video resolution"));
				}

				const resolution = Object.entries(RESOLUTIONS).find(
					([, { width: w, height: h }]) => w === width && h === height
				);

				resolve(
					resolution
						? (resolution[0] as unknown as Resolutions)
						: Resolutions["P720"]
				);
			});
		});
	}

	kill() {
		if (!this.#command) {
			console.log("[skipped]: no ffmpeg process to kill");
			return;
		}

		this.#command.kill("SIGTERM");

		console.log("[killed]: ffmpeg process");
	}

	async process() {
		const done: string[] = [];

		const item = this.#source;

		// Got the output folder name from the filename
		const outputFolder = this.#getOutputFolder(item.filename);

		// make the output folder
		await mkdir(outputFolder, { recursive: true });

		const success = await this.#transcodeResolutions(item, outputFolder);

		if (success) done.push(item.file);

		await this.#generateAnimatedWebp(item, outputFolder);

		console.log(`[finished]: ${done.length} file(s) successfully processed`);
	}

	async #transcodeResolutions(source: TranscoderSource, outputFolder: string) {
		if (!this.#resolutions.length) return true;

		const resolutionPlaylists: TranscoderPlaylist[] = [];

		for (const resolution of this.#resolutions) {
			console.log(
				`[started]: processing ${resolution}p for ${source.filename}`
			);
			logger.step({
				index: 1,
				process: `Transcoding ${resolution}p`,
				file: source.file,
			});

			const playlist = await this.#transcode(source, resolution, outputFolder);

			if (!playlist) {
				console.log(
					`[skipping]: ${resolution}p for ${source.filename}; no playlist returned`
				);
				continue;
			}

			resolutionPlaylists.push(playlist);

			console.log(
				`[completed]: processing ${resolution}p for ${source.filename}`
			);
		}

		return this.#buildMainPlaylist(resolutionPlaylists, outputFolder);
	}

	async #transcode(
		{ file, filename }: TranscoderSource,
		resolution: Resolutions,
		outputFolder: string
	): Promise<TranscoderPlaylist | null> {
		const resolutionOutput = `${outputFolder}/${resolution}p`;
		const filenameLessExt = filename.split(".").shift() as string;
		const outputFilenameLessExt = `${filenameLessExt}_${resolution}`;
		const outputPlaylist = `${resolutionOutput}/${outputFilenameLessExt}p.m3u8`;
		const outputSegment = `${resolutionOutput}/${outputFilenameLessExt}_%03d.ts`;
		const outputPlaylisFromMain = `${resolution}p/${outputFilenameLessExt}p.m3u8`;
		const { height, bitrate } = RESOLUTIONS.get(resolution) ?? {};

		if (!height || !bitrate) {
			console.error(
				`[argument error]: Invalid resolution provided: ${resolution}`
			);
			return null;
		}

		await mkdir(resolutionOutput, { recursive: true });

		return new Promise((resolve) => {
			this.#command = ffmpeg(decodeURI(file))
				.output(outputPlaylist)
				.videoCodec("libx264")
				// .videoBitrate(`${bitrate}k`)
				.audioCodec("aac")
				.audioBitrate("148k")
				.outputOptions([
					"-filter:v",
					`scale=-2:${height}`,
					"-preset",
					"fast",
					"-crf",
					"28",
					"-hls_time",
					"4",
					"-hls_playlist_type",
					"vod",
					"-hls_segment_filename",
					outputSegment,
				]);

			this.#command.on("progress", (progress) => {
				logger.progress({ file, percent: progress.percent });
				console.log(
					`[progress]: ${progress.percent?.toFixed(2)}% @ frame ${
						progress.frames
					}; timemark ${progress.timemark}`
				);
			});

			this.#command.on("start", () => {
				console.log(`[started]: transcoding ${resolution}p for ${filename}`);
				logger.progress({ file, percent: 0 });
			});

			this.#command.on("end", async () => {
				console.log(
					`[completed]: transcoding ${resolution}p for ${filename}; output ${outputPlaylist}`
				);
				logger.progress({ file, percent: 100 });
				resolve({
					resolution: await this.#detectPlaylistResolution(outputPlaylist),
					playlistFilename: outputPlaylist.split("/").pop() as string,
					playlistPathFromMain: outputPlaylisFromMain,
					playlistPath: outputPlaylist,
					bitrate,
				});
			});

			this.#command.on("error", (err) =>
				this.#onFfmpegError(file, err, resolve)
			);
			this.#command.run();
		});
	}

	async #generateAnimatedWebp(
		{ file, filename }: TranscoderSource,
		outputFolder: string
	): Promise<string | null> {
		if (!this.#includeWebp) return null;

		const output = `${outputFolder}/video.webp`;
		const duration = await this.#detectVideoDuration(file);

		logger.step({ index: 3, process: `Generating Animated WebP`, file });

		return new Promise((resolve) => {
			this.#command = ffmpeg(decodeURI(file))
				.output(output)
				.videoCodec("libwebp")
				.videoFilter(`fps=30, scale=320:-1`)
				.setStartTime(duration > 30 ? 10 : 0)
				.duration(6)
				.outputOptions(["-preset", "picture", "-loop", "0", "-an"]);

			this.#command.on("progress", (progress) => {
				// get percentage done from progress timemark in format HH:MM:SS.000 compared to duration
				const percent = (Number(progress.timemark.split(":").pop()) / 6) * 100;
				logger.progress({ file, percent });
				console.log(
					`[progress]: ${percent?.toFixed(2)}% @ frame ${
						progress.frames
					}; timemark ${progress.timemark}`
				);
			});

			this.#command.on("start", () => {
				console.log(`[started]: generating animated webp for ${filename}`);
				logger.progress({ file, percent: 0 });
			});

			this.#command.on("end", async () => {
				logger.progress({ file, percent: 100 });
				console.log(
					`[completed]: generating animated webp for ${filename}; output ${output}`
				);
				resolve(output);
			});

			this.#command.on("error", (err) =>
				this.#onFfmpegError(file, err, resolve)
			);
			this.#command.run();
		});
	}

	#onFfmpegError(file: string, err: Error, resolve: (value: null) => void) {
		logger.progress({ file, percent: -1 });
		console.error(`[ffmpeg error]: ${err.message}`);
		resolve(null);
	}

	async #buildMainPlaylist(
		playlists: TranscoderPlaylist[],
		outputFolder: string
	) {
		if (!playlists.length) {
			console.log(
				`[skipping]: main playlist for ${outputFolder}; no resolution playlists found`
			);
			return;
		}

		console.log(
			`[started]: generating main playlist ${outputFolder}/main.m3u8`
		);

		const main = ["#EXTM3U", "#EXT-X-VERSION:3"];

		for (const playlist of playlists) {
			console.log(
				`[playlist]: ${playlist.resolution.height}p for ${playlist.playlistPathFromMain}`
			);
			main.push(
				`#EXT-X-STREAM-INF:BANDWIDTH=${playlist.bitrate * 1000},RESOLUTION=${
					playlist.resolution.width
				}x${playlist.resolution.height}`
			);
			main.push(playlist.playlistPathFromMain);
		}

		const final = main.join("\n");

		await writeFile(`${outputFolder}/main.m3u8`, final);

		console.log(
			`[completed]: generating main playlist ${outputFolder}/main.m3u8`
		);

		return true;
	}

	async #detectPlaylistResolution(
		playlistPath: string
	): Promise<{ width: number; height: number }> {
		return new Promise((resolve, reject) => {
			ffmpeg(playlistPath).ffprobe((err, data) => {
				if (err) {
					return reject(err);
				}

				const { width, height } =
					data.streams.find((stream) => stream.codec_type === "video") ?? {};

				if (!width || !height) {
					return reject(new Error("Could not detect playlist resolution"));
				}

				resolve({ width, height });
			});
		});
	}

	async #detectVideoDuration(path: string): Promise<number> {
		return new Promise((resolve, reject) => {
			ffmpeg(path).ffprobe((err, data) => {
				if (err) {
					return reject(err);
				}

				const { duration } =
					data.streams.find((stream) => stream.codec_type === "video") ?? {};

				if (!duration) {
					return reject(new Error("Could not detect playlist resolution"));
				}

				resolve(Number(duration));
			});
		});
	}

	#getOutputFolder(filename: string) {
		const filenameLessExt = filename.split(".").shift() as string;
		return `${this.#output}/${filenameLessExt}`;
	}
}
