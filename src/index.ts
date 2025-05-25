import Transcoder from "@/services/transcoder";

async function main() {
	const transcoder = new Transcoder();

	const startTime = performance.now();
	await transcoder.process();
	const endTime = performance.now();

	console.log(
		`Process completed in ${
			Math.round((endTime - startTime) / 1000) || 0
		} seconds`
	);
}

main();
