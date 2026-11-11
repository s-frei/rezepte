#!/usr/bin/env bun
//MISE description="Fail on British spellings in tracked text files"
// The rule and its exceptions: docs/memory/content/conventions/language.mdx.
import { git } from '../../lib/release/git';
import { excluded, hits } from '../../lib/guards/spelling';

const cwd = process.cwd();
const found: string[] = [];
for (const path of git(['ls-files', '-z'], cwd).split('\0')) {
	if (!path || excluded(path)) continue;
	const file = Bun.file(path);
	if (!(await file.exists())) continue; // deleted but not yet staged
	const bytes = await file.bytes();
	// Binary files: a NUL byte in the head, the test git itself uses.
	if (bytes.subarray(0, 8000).includes(0)) continue;
	for (const h of hits(new TextDecoder().decode(bytes))) found.push(`${path}:${h.line}:${h.word}`);
}
if (found.length) {
	console.error(`spelling:check: British spellings, write the American ones (docs/memory/content/conventions/language.mdx):\n${found.join('\n')}`);
	process.exit(1);
}
console.log('spelling:check: no British spellings');
