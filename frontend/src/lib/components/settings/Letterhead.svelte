<script lang="ts" module>
	export type LetterheadLine = { key: string; main: string; sub?: string; url?: boolean };

	/**
	 * A URL cut after every path slash, so a `<wbr>` between the pieces lets
	 * it break there and nowhere else; the scheme stays with the host.
	 */
	export function urlSegments(url: string): string[] {
		const scheme = /^[a-z][a-z\d+.-]*:\/\//i.exec(url)?.[0] ?? '';
		const segments = url.slice(scheme.length).split(/(?<=\/)/);
		segments[0] = scheme + segments[0];
		return segments;
	}
</script>

<script lang="ts">
	let {
		stamp,
		lines,
		blank
	}: { stamp: { label: string; on: boolean }; lines: LetterheadLine[]; blank: boolean } = $props();

	// One layout for every row: if any value does not fit beside its key and
	// leader, all of them move under their leaders. Measured in the one-line
	// layout each time, so widening the card brings it back.
	function fitRows(dl: HTMLElement) {
		void lines; // measure again when the values change
		const measure = () => {
			dl.removeAttribute('data-stacked');
			const overflows = [...dl.children].some((row) => row.scrollWidth > row.clientWidth);
			dl.toggleAttribute('data-stacked', overflows);
		};
		measure();
		const observer = new ResizeObserver(measure);
		observer.observe(dl);
		return () => observer.disconnect();
	}
</script>

{#snippet text(value: string, url: boolean | undefined)}
	{#if url}
		{#each urlSegments(value) as segment, i (i)}{segment}<wbr />{/each}
	{:else}
		{value}
	{/if}
{/snippet}

<!-- A letterhead: the same lines whether set up or not, so the card keeps
     its shape when it is turned on and only the lines fill in. Each value
     is a main part and a smaller line under it. Stacked, the values sit
     left under their key; a value breaks inside a word only when nothing
     else lets it fit (an address on a 320px phone). -->
<div class="relative mt-6 rounded-lg border border-border bg-surface-elevated px-5 py-4">
	<span
		class="absolute -top-2.5 right-4 inline-flex items-center gap-1.5 rounded-pill border border-border bg-surface px-2.5 py-0.5 text-label font-bold text-text-muted uppercase"
	>
		{#if stamp.on}
			<span aria-hidden="true" class="size-1.5 rounded-full bg-success"></span>
			<span class="text-text">{stamp.label}</span>
		{:else}
			{stamp.label}
		{/if}
	</span>
	<dl {@attach fitRows} class="group">
		{#each lines as { key, main, sub, url } (key)}
			<div class="flex items-baseline gap-x-2.5 py-1 group-data-stacked:flex-wrap">
				<dt
					class="shrink-0 font-display text-body-lg whitespace-nowrap italic {blank
						? 'text-text-muted'
						: 'text-primary'}"
				>
					{key}
				</dt>
				<span
					aria-hidden="true"
					class="min-w-6 flex-1 -translate-y-1 border-b-2 border-dotted border-border"
				></span>
				<dd
					class="shrink-0 text-right text-body whitespace-nowrap group-data-stacked:basis-full group-data-stacked:text-left group-data-stacked:wrap-anywhere group-data-stacked:whitespace-normal {blank
						? 'text-text-muted'
						: ''}"
				>
					<span class="block">{@render text(main, url)}</span>
					{#if sub}
						<span class="block text-caption text-text-muted">{@render text(sub, url)}</span>
					{/if}
				</dd>
			</div>
		{/each}
	</dl>
</div>
