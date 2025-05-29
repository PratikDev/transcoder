import ffmpeg from "fluent-ffmpeg";
import { mkdir, writeFile } from "node:fs/promises";

export type TranscoderSource = {
	file: string;
	filename: string;
	extname: string;
};

export type TranscoderPlaylist = {
	resolution: { width: number; height: number };
	bitrate: number;
	playlistFilename: string;
	playlistPathFromMain: string;
	playlistPath: string;
};

export enum Resolutions {
	"P2160" = 2160,
	"P1440" = 1440,
	"P1080" = 1080,
	"P720" = 720,
	"P480" = 480,
	"P360" = 360,
}

export const RESOLUTIONS = new Map([
	[Resolutions.P2160, { height: 2160, width: 3840, bitrate: 14_000 }],
	[Resolutions.P1440, { height: 1440, width: 2560, bitrate: 9_000 }],
	[Resolutions.P1080, { height: 1080, width: 1920, bitrate: 6_500 }],
	[Resolutions.P720, { height: 720, width: 1280, bitrate: 4_000 }],
	[Resolutions.P480, { height: 480, width: 854, bitrate: 2_000 }],
	[Resolutions.P360, { height: 360, width: 640, bitrate: 1_000 }],
]);

export default class Transcoder {
	#source: TranscoderSource = {
		file: "./assets/video.mp4",
		filename: "video.mp4",
		extname: "mp4",
	};
	#resolutions: Resolutions[] = [720, 480, 360];
	#output: string = "./output";

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

	async process() {
		const item = this.#source;

		// Got the output folder name from the file and output dir name
		const outputFolder = `${this.#output}/${item.filename.split(".").shift()}`;

		// make the output folder
		await mkdir(outputFolder, { recursive: true });

		const success = await this.#transcodeResolutions(item, outputFolder);
		if (!success) {
			console.log(`[failed]: transcoding for ${item.filename} failed`);
			return;
		}

		console.log(
			`[finished]: ${
				item.filename
			} file successfully processed for ${this.#resolutions.join(
				", "
			)} resolutions`
		);
	}

	async #transcodeResolutions(source: TranscoderSource, outputFolder: string) {
		const resolutionPlaylists: TranscoderPlaylist[] = [];

		for (const resolution of this.#resolutions) {
			const playlist = await this.#transcode(source, resolution, outputFolder);
			if (!playlist) {
				console.log(
					`[skipping]: ${resolution}p for ${source.filename}; no playlist returned`
				);
				continue;
			}

			resolutionPlaylists.push(playlist);
		}

		return this.#buildMainPlaylist(resolutionPlaylists, outputFolder);
	}

	async #transcode(
		{ file, filename }: TranscoderSource,
		resolution: Resolutions,
		outputFolder: string
	): Promise<TranscoderPlaylist | null> {
		// Validate the resolution
		const { height, bitrate } = RESOLUTIONS.get(resolution) ?? {};
		if (!height || !bitrate) {
			console.error(
				`[argument error]: Invalid resolution provided: ${resolution}`
			);
			return null;
		}

		const resolutionOutput = `${outputFolder}/${resolution}p`;
		const filenameLessExt = filename.split(".").shift() as string;
		const outputFilenameLessExt = `${filenameLessExt}_${resolution}`;
		const outputPlaylist = `${resolutionOutput}/${outputFilenameLessExt}p.m3u8`;
		const outputSegment = `${resolutionOutput}/${outputFilenameLessExt}_%03d.ts`;
		const outputPlaylisFromMain = `${resolution}p/${outputFilenameLessExt}p.m3u8`;

		await mkdir(resolutionOutput, { recursive: true });

		return new Promise((resolve) => {
			ffmpeg(decodeURI(file))
				.output(outputPlaylist)
				.videoCodec("libx264")
				.videoBitrate(`${bitrate}k`)
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
				])
				.on("start", () => {
					console.log(`[started]: transcoding ${resolution}p for ${filename}`);
				})
				.on("progress", (progress) => {
					console.log(
						`[progress]: ${progress.percent?.toFixed(2)}% @ frame ${
							progress.frames
						}; timemark ${progress.timemark}`
					);
				})
				.on("end", async () => {
					console.log(
						`[completed]: transcoding ${resolution}p for ${filename}; output ${outputPlaylist}`
					);
					resolve({
						resolution: await this.#detectPlaylistResolution(outputPlaylist),
						playlistFilename: outputPlaylist.split("/").pop() as string,
						playlistPathFromMain: outputPlaylisFromMain,
						playlistPath: outputPlaylist,
						bitrate,
					});
				})
				.on("error", (err) => {
					console.error(`[ffmpeg error]: ${err.message}`);
					resolve(null);
				})
				.run();
		});
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
}
