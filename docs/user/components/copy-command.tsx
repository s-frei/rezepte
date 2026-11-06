'use client';

import { useCopyButton } from 'fumadocs-ui/utils/use-copy-button';
import { Check, Copy } from 'lucide-react';

export function CopyCommand({ command }: { command: string }) {
	const [checked, onClick] = useCopyButton(() => navigator.clipboard.writeText(command));
	return (
		<button
			type="button"
			onClick={onClick}
			aria-label={checked ? 'Copied' : 'Copy command'}
			className="shrink-0 rounded-md p-1.5 text-fd-muted-foreground transition-colors hover:bg-fd-accent hover:text-fd-accent-foreground"
		>
			{checked ? <Check className="size-4" /> : <Copy className="size-4" />}
		</button>
	);
}
