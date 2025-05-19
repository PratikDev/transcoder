import Transcoder from "@/services/transcoder";

const output = "./output";
const includeWebp = false;
const _resolutions = "720,480,360";
const _source = "./assets/video.mp4";

async function main() {
	const resolutions = Transcoder.parseResolutions(_resolutions);
	const source = Transcoder.parseSourceFile(_source);

	if (!output || !source || !resolutions.length) {
		console.error("missing required argument(s)");
		return process.exit(1);
	}

	console.log(
		`[running]: transcode for file: ${source.file} with ${
			resolutions.join(", ") || "N/A"
		} resolutions`
	);

	const transcoder = new Transcoder({
		resolutions,
		output,
		source,
		includeWebp,
	});

	const startTime = performance.now();
	await transcoder.process();
	const endTime = performance.now();

	console.log(
		`[completed]: transcode for file: ${source.file} in ${
			resolutions.join(", ") || "N/A"
		} resolutions in ${Math.round((endTime - startTime) / 1000) || 0} seconds`
	);

	process.exit(0);
}

main();
