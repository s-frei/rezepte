<script lang="ts">
	import CalendarDays from '@lucide/svelte/icons/calendar-days';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { Popover } from 'bits-ui';
	import { m } from '#lib/paraglide/messages.js';
	import { fitStamps, type StampFit } from '#lib/shopping/stamp-fit.js';
	import LinkStamp, { stampTone, type StampProps } from './LinkStamp.svelte';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';

	/**
	 * Stamps in a row: the one fit rule (`fitStamps`). Measured by width, up
	 * to `lines` lines: all whole when they fit, else all without their
	 * detail ("· 10 offen", "· Mo") if that shows more; a name is never cut
	 * and never wraps. What does not fit goes behind a "+n" on the last
	 * stamp's line, which opens the rest in a popover; when not even the
	 * first bare name fits, the row is one "3 Listen" in its kind's tint that
	 * opens them all. Accessible names keep every detail. Fitted before the
	 * first paint, again once the fonts are in and when the row's width or a
	 * stamp's text changes.
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

	const GAP = 6;
	let row = $state<HTMLUListElement>();
	let meter = $state<HTMLElement>();
	let content = $state<HTMLElement | null>(null);
	/** Until the first fit the row keeps its place but shows nothing, so no stamp flashes past. */
	let measured = $state(false);
	let fitted = $state<StampFit>({ shown: Infinity, bare: false, summary: false });
	const visible = $derived(stamps.slice(0, fitted.shown));
	const rest = $derived(stamps.slice(visible.length));
	const plans = $derived(stamps[0]?.kind === 'plan');
	// The summary pill: "5 Listen", named "5 Listen zeigen" like the "+n".
	const all = $derived(
		rest.length === 1
			? plans
				? m.link_all_plans_one()
				: m.link_all_lists_one()
			: plans
				? m.link_all_plans({ count: rest.length })
				: m.link_all_lists({ count: rest.length })
	);
	// Names what the "+n" holds: "2 weitere Listen zeigen".
	const more = $derived(
		visible.length === 0
			? rest.length === 1
				? plans
					? m.link_all_plans_one_show()
					: m.link_all_lists_one_show()
				: plans
					? m.link_all_plans_show({ count: rest.length })
					: m.link_all_lists_show({ count: rest.length })
			: plans
				? rest.length === 1
					? m.link_more_plans_one()
					: m.link_more_plans({ count: rest.length })
				: rest.length === 1
					? m.link_more_lists_one()
					: m.link_more_lists({ count: rest.length })
	);

	/** Fits the row from the hidden meter's widths: each stamp whole and bare, and its largest "+n". */
	function fit() {
		if (!row || !meter) return;
		const width = (el: Element | null) => el?.getBoundingClientRect().width ?? 0;
		const widths = [...meter.querySelectorAll('[data-meter-stamp]')].map((el) => ({
			full: width(el.querySelector('[data-full]')),
			bare: width(el.querySelector('[data-bare]'))
		}));
		// Sub-pixel widths add up: half a pixel of slack keeps the last stamp from spilling over.
		fitted = fitStamps(
			widths,
			row.getBoundingClientRect().width - 0.5,
			lines,
			GAP,
			width(meter.querySelector('[data-meter-more]'))
		);
		measured = true;
	}

	$effect(() => {
		void lines;
		void stamps.map((s) => `${s.key}|${s.label}|${s.day}|${s.open}`).join();
		if (!row) return;
		// Before the first paint; again once the web fonts have their widths.
		fit();
		let live = true;
		void document.fonts?.ready.then(() => live && fit());
		// A face that starts loading later (first used by this text) refits too.
		document.fonts?.addEventListener('loadingdone', fit);
		let seen = row.getBoundingClientRect().width;
		const observer = new ResizeObserver(([e]) => {
			if (Math.abs(e.contentRect.width - seen) < 1) return;
			seen = e.contentRect.width;
			fit();
		});
		observer.observe(row);
		return () => {
			live = false;
			document.fonts?.removeEventListener('loadingdone', fit);
			observer.disconnect();
		};
	});
</script>

{#snippet moreButton()}
	<Popover.Root>
		<Popover.Trigger
			aria-label={more}
			data-stamp
			class="relative inline-flex h-6 shrink-0 items-center gap-1 rounded-pill px-2 text-caption font-semibold transition hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary max-md:after:absolute max-md:after:-inset-x-2 max-md:after:inset-y-[calc(50%-22px)] {visible.length ===
			0
				? stampTone(stamps[0]?.kind ?? 'list')
				: 'ring-1 ring-border ring-inset hover:bg-background'} {stampClass}"
			>{#if visible.length === 0}{#if plans}<CalendarDays
						class="size-3.5 shrink-0"
						aria-hidden="true"
					/>{:else}<ShoppingBasket
						class="size-3.5 shrink-0"
						aria-hidden="true"
					/>{/if}{all}<ChevronDown
					class="size-3 shrink-0"
					aria-hidden="true"
				/>{:else}+{rest.length}{/if}</Popover.Trigger
		>
		<Popover.Portal>
			<!-- Opened by a tap, no stamp lights up as if picked: the popup itself
			     takes the focus, and Tab moves into its stamps. -->
			<Popover.Content
				bind:ref={content}
				tabindex={-1}
				onOpenAutoFocus={(event) => {
					event.preventDefault();
					content?.focus();
				}}
				role="dialog"
				aria-label={label ?? more}
				side="bottom"
				align="start"
				sideOffset={6}
				collisionPadding={12}
				class="z-50 max-w-[calc(100vw-24px)] rounded-2xl border border-popover-border bg-popover p-3 shadow-dialog outline-none"
			>
				<ul class="flex flex-col items-start gap-2.5">
					{#each rest as stamp (stamp.key)}
						<li class="flex max-w-full min-w-0">
							<!-- In the popover a long name may wrap, its detail running on after it: nothing here is cut.
							     Outside a card's drop zone, a real link again. -->
							<LinkStamp
								{...stamp}
								card={false}
								wrap
								class="{stampClass} h-auto! min-h-6 rounded-lg! py-1"
							/>
						</li>
					{/each}
				</ul>
			</Popover.Content>
		</Popover.Portal>
	</Popover.Root>
{/snippet}

<ul
	bind:this={row}
	aria-label={label}
	class="relative flex min-h-6 min-w-0 items-center gap-1.5 {lines === 1
		? 'flex-nowrap'
		: 'flex-wrap'} {measured ? '' : 'invisible'} {extra}"
>
	{#each visible as stamp, i (stamp.key)}
		<!-- The "+n" keeps to the last stamp, on its line. -->
		<li data-stamp-item class="flex shrink-0 items-center gap-1.5">
			<LinkStamp {...stamp} bare={fitted.bare} class={stampClass} />
			{#if rest.length > 0 && i === visible.length - 1}{@render moreButton()}{/if}
		</li>
	{/each}
	{#if fitted.summary}
		<li class="flex shrink-0">{@render moreButton()}</li>
	{/if}
	<!-- What the row measures: each stamp whole and bare, and a "+n". -->
	<li
		bind:this={meter}
		aria-hidden="true"
		class="pointer-events-none invisible absolute top-0 left-0 flex h-0 w-max flex-col items-start overflow-hidden"
	>
		{#each stamps as stamp (stamp.key)}
			<span data-meter-stamp class="flex">
				<span data-full class="flex"><LinkStamp {...stamp} href={null} class={stampClass} /></span>
				<span data-bare class="flex"
					><LinkStamp {...stamp} href={null} bare class={stampClass} /></span
				>
			</span>
		{/each}
		<span
			data-meter-more
			class="inline-flex h-6 items-center rounded-pill px-2 text-caption font-semibold {stampClass}"
			>+{stamps.length}</span
		>
	</li>
</ul>
