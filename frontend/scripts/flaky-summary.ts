// Lists the e2e tests that failed once and passed on CI's retry in the job
// summary, so a flake shows on the run's page instead of hiding in a green
// log. Reads the JSON report playwright.config.ts writes on CI.
import { appendFile } from 'node:fs/promises';

type Suite = {
	specs?: {
		file: string;
		line: number;
		title: string;
		tests: { status: string; projectName: string }[];
	}[];
	suites?: Suite[];
};

export function flaky(suites: Suite[]): string[] {
	return suites.flatMap((s) => [
		...(s.specs ?? []).flatMap((spec) =>
			spec.tests
				.filter((t) => t.status === 'flaky')
				.map((t) => `${spec.file}:${spec.line} › ${spec.title} [${t.projectName}]`)
		),
		...flaky(s.suites ?? [])
	]);
}

if (import.meta.main) {
	const report = Bun.file('playwright-results.json');
	const found = (await report.exists()) ? flaky((await report.json()).suites) : [];
	const summary = process.env.GITHUB_STEP_SUMMARY;
	if (found.length && summary) {
		await appendFile(
			summary,
			`### ${found.length} flaky e2e test(s)\n\nFailed once, passed on the retry:\n\n${found.map((f) => `- \`${f}\``).join('\n')}\n`
		);
	}
	for (const f of found) console.log(`flaky: ${f}`);
}
