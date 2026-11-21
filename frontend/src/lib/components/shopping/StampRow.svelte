<script lang="ts">
	import { Popover } from 'bits-ui';
	import { tick } from 'svelte';
	import { m } from '#lib/paraglide/messages.js';
	import LinkStamp, { type StampProps } from './LinkStamp.svelte';

	/**
	 * Stamps in a row: the one overflow rule. They wrap, measured by width,
	 * up to `lines` lines; what does not fit goes behind a "+n" that opens
	 * the rest in a popover. On one line the last stamp left truncates
	 * rather than drop below its "+n". Remeasured when the row's width or
	 * a stamp's text changes.
	 */
	let {
		stamps,
		lines = 2,
		label,
		class: extra = '',
		stampClass = ''
	}: {
		stamps: (StampProps & { key: string })[];
		lines?: number;
		/** Names the row for assistive technology ("Aus Plänen"). */
		label?: string;
		class?: string;
		stampClass?: string;
	} = $props();

	let row = $state<HTMLUListElement>();
	let shown = $state(Infinity);
	const visible = $derived(stamps.slice(0, shown));
	const rest = $derived(stamps.slice(visible.length));
	// Names what the "+n" holds: "2 weitere Listen zeigen".
	const more = $derived(
		stamps[0]?.kind === 'plan'
			? rest.length === 1
				? m.link_more_plans_one()
				: m.link_more_plans({ count: rest.length })
			: rest.length === 1
				? m.link_more_lists_one()
				: m.link_more_lists({ count: rest.length })
	);

	/** Distinct line tops of the row's items. */
	const tops = (ul: HTMLUListElement) =>
		[...new Set([...ul.children].map((li) => Math.round((li as HTMLElement).offsetTop)))].sort(
			(a, b) => a - b
		);

	let run = 0;
	async function fit() {
		const mine = ++run;
		shown = Infinity;
		await tick();
		while (row && mine === run) {
			let next: number;
			if (lines === 1) {
				if (row.scrollWidth <= row.clientWidth + 1) return;
				next = visible.length - 1;
			} else {
				const lineTops = tops(row);
				if (lineTops.length <= lines) return;
				const limit = lineTops[lines - 1];
				const fits = [...row.querySelectorAll<HTMLElement>('[data-stamp-item]')].filter(
					(li) => Math.round(li.offsetTop) <= limit
				).length;
				next = Math.min(visible.length - 1, fits);
			}
			if (next < 1) {
				shown = 1;
				return;
			}
			shown = next;
			await tick();
		}
	}

	$effect(() => {
		void lines;
		void stamps.map((s) => `${s.key}|${s.label}|${s.day}|${s.open}`).join();
		if (!row) return;
		let width = 0;
		const observer = new ResizeObserver(([e]) => {
			if (Math.abs(e.contentRect.width - width) < 1) return;
			width = e.contentRect.width;
			void fit();
		});
		observer.observe(row);
		return () => observer.disconnect();
	});
</script>

<ul
	bind:this={row}
	aria-label={label}
	class="flex min-w-0 items-center gap-1.5 {lines === 1 ? 'flex-nowrap' : 'flex-wrap'} {extra}"
>
	{#each visible as stamp (stamp.key)}
		<li
			data-stamp-item
			class="flex max-w-full min-w-0 {lines === 1 && visible.length > 1 ? 'shrink-0' : ''}"
		>
			<LinkStamp {...stamp} class={stampClass} />
		</li>
	{/each}
	{#if rest.length > 0}
		<li class="flex shrink-0">
			<Popover.Root>
				<Popover.Trigger
					aria-label={more}
					data-stamp
					class="relative inline-flex h-6 items-center rounded-pill px-2 text-caption font-semibold ring-1 ring-border transition ring-inset hover:bg-background focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:-inset-x-2 max-md:after:inset-y-[calc(50%-22px)] {stampClass}"
					>+{rest.length}</Popover.Trigger
				>
				<Popover.Portal>
					<Popover.Content
						side="bottom"
						align="start"
						sideOffset={6}
						collisionPadding={12}
						class="z-50 max-w-[min(20rem,calc(100vw-24px))] rounded-2xl border border-popover-border bg-popover p-3 shadow-dialog"
					>
						<ul class="flex flex-col items-start gap-2.5">
							{#each rest as stamp (stamp.key)}
								<li class="flex max-w-full min-w-0">
									<LinkStamp
										{...stamp}
										class="{stampClass} h-auto! min-h-6 rounded-lg! py-1 whitespace-normal [&_.truncate]:whitespace-normal"
									/>
								</li>
							{/each}
						</ul>
					</Popover.Content>
				</Popover.Portal>
			</Popover.Root>
		</li>
	{/if}
</ul>
