import { LogProgressStatus, LogTypes } from "@/constant/logger";
import type { Progress, Step } from "@/types/transcoder";

class Logger {
	progress(data: Progress) {
		if (!data.status) {
			if (data.percent === 100) data.status = LogProgressStatus.DONE;
			else if (data.percent) data.status = LogProgressStatus.WORKING;
			else data.status = LogProgressStatus.QUEUED;
		}

		console.log(JSON.stringify([LogTypes.PROGRESS, data]));
	}

	step(data: Step) {
		console.log(JSON.stringify([LogTypes.STEP, data]));
	}
}

const logger = new Logger();
export default logger;
