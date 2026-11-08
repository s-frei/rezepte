<script lang="ts">
	import { RadioGroup } from 'bits-ui';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Copy from '@lucide/svelte/icons/copy';
	import Globe from '@lucide/svelte/icons/globe';
	import House from '@lucide/svelte/icons/house';
	import Lock from '@lucide/svelte/icons/lock';
	import Scissors from '@lucide/svelte/icons/scissors';
	import Share from '@lucide/svelte/icons/share';
	import TicketPlus from '@lucide/svelte/icons/ticket-plus';
	import TicketX from '@lucide/svelte/icons/ticket-x';
	import Tickets from '@lucide/svelte/icons/tickets';
	import Users from '@lucide/svelte/icons/users';
	import { flushSync, tick, untrack, type Snippet } from 'svelte';
	import { prefersReducedMotion } from 'svelte/motion';
	import { slide } from 'svelte/transition';
	import { toast } from 'svelte-sonner';
	import { resolve } from '$app/paths';
	import { ApiError } from '#lib/api/client.js';
	import type { Recipe } from '#lib/api/recipes.js';
	import {
		createPublicShare,
		retimeShare,
		revokeShare,
		type PublicShare,
		type ShareLifetime
	} from '#lib/api/shares.js';
	import Button from '#lib/components/ui/Button.svelte';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { lifetimeOptions } from '#lib/recipe/lifetimes.js';
	import { canShareLink, copyLink, shareLink } from '#lib/recipe/share.svelte.js';
	import { stubFace, type StubFace } from '#lib/recipe/share-stub.js';
	import LifetimePicker from './LifetimePicker.svelte';
	import ShareTicket from './ShareTicket.svelte';

	type Audience = 'household' | 'everyone';

	// The pass-on sheet's Link tab: the ticket, the choice between the
	// household address and a public link, and what creates, re-times,
	// revokes and hands on that link. It mounts with the sheet, so every
	// opening starts afresh: on the public link when it is active, else on
	// the household.
	//
	// On a phone it is one column in the sheet's tab cell, with its own
	// button row; from 768px it is the dialog's two columns, the switch and
	// the heading lent by the sheet, drawn only while the tab shows.
	let {
		recipe,
		linkUrl,
		share = $bindable(null),
		canCreateShare,
		defaultDays,
		maxDays,
		phone,
		shown,
		switcher,
		heading
	}: {
		recipe: Recipe;
		/** The household address - the page's `ShareLink`, read at tap time. */
		linkUrl: () => string;
		/** The member's own public link to this recipe. */
		share?: PublicShare | null;
		/** Public sharing is on and this member may use it. */
		canCreateShare: boolean;
		defaultDays: ShareLifetime;
		maxDays: ShareLifetime;
		phone: boolean;
		/** The Link tab is the one showing. */
		shown: boolean;
		/** The dialog's tab switch, at the top of its left column. */
		switcher: Snippet;
		/** The dialog's label and title, at the top of its right column. */
		heading: Snippet;
	} = $props();

	const DAY = 86_400_000;
	const uid = $props.id();
	const now = new Date();
	const options = $derived(lifetimeOptions(maxDays));
	// Only a member who may share publicly, or who already has a link, gets the choice.
	const offered = $derived(canCreateShare || share !== null);

	let audience = $state<Audience>(
		untrack(() => (share?.status === 'active' ? 'everyone' : 'household'))
	);
	// The lifetime Create uses: the owner's default, clamped to the maximum.
	let pending = $state<ShareLifetime>(
		untrack(() => (options.includes(defaultDays) ? defaultDays : (options.at(-1) ?? null)))
	);
	let creating = $state(false);
	let retiming = $state(false);
	let revokeOpen = $state(false);
	// The preview's fit step on a phone, see `fitRegion`.
	let fit = $state(0);
	// How far the ticket shrinks at the last fit step, as a CSS zoom.
	let shrink = $state(1);
	// The chosen row's detail unfolds, the other one's folds away.
	const unfold = $derived(prefersReducedMotion.current ? 0 : 240);
	// The two details' footers: the perforation and the links behind it.
	const footer =
		'flex flex-wrap items-center justify-between gap-x-2 border-t-2 border-dashed border-border pt-1 text-caption font-semibold whitespace-nowrap';
	const footerLink =
		'inline-flex h-8.5 items-center gap-1.5 rounded-sm focus-visible:outline-2 focus-visible:outline-primary';

	const everyone = $derived(offered && audience === 'everyone');
	// The link as it stands, or as Create would make it.
	const link = $derived(share ?? { status: 'active' as const, expiresAt: expiry(pending) });
	const face = $derived<StubFace>(everyone ? stubFace(link, now) : { kind: 'household' });
	// Once the link exists, its cell is the lifetime it was last given.
	const lifetime = $derived(share ? lifetimeOf(share) : pending);
	// A paused or limited link does not open: nothing hands it on.
	const sendable = $derived(!everyone || share?.status === 'active');
	// The primary creates the link first; once it exists the same button
	// hands it on, so focus stays on it.
	const creatable = $derived(everyone && !share);

	function expiry(days: ShareLifetime): string | null {
		return days === null ? null : new Date(now.getTime() + days * DAY).toISOString();
	}

	function lifetimeOf(it: PublicShare): ShareLifetime {
		if (it.expiresAt === null) return null;
		return Math.round((Date.parse(it.expiresAt) - Date.parse(it.startsAt)) / DAY) as ShareLifetime;
	}

	// Day and short month, the year only when it is not this one; never
	// broken across lines.
	function shortDate(iso: string): string {
		const date = new Date(iso);
		return new Intl.DateTimeFormat(getLocale(), {
			day: 'numeric',
			month: 'short',
			year: date.getFullYear() === now.getFullYear() ? undefined : 'numeric'
		})
			.format(date)
			.replaceAll(' ', '\u00a0');
	}

	// What the ticket shows: host and path, the way a chat shows a link.
	function shownAddress(): string {
		if (everyone) return location.host + (share?.path ?? '/s/…');
		const url = new URL(linkUrl());
		return url.host + url.pathname;
	}

	// What Share link and Copy hand on.
	function address(): string {
		return everyone && share ? location.origin + share.path : linkUrl();
	}

	// A refusal the service gives create and re-time alike.
	function refused(error: unknown, fallback: string) {
		if (error instanceof ApiError && error.status === 403) {
			toast.error(
				error.body.reason === 'sharing-off'
					? m.public_share_error_off()
					: m.public_share_error_not_allowed()
			);
		} else if (error instanceof ApiError && error.status === 422) {
			toast.error(m.public_share_error_lifetime());
		} else {
			toast.error(fallback);
		}
	}

	// The link is gone and with it what was focused (Revoke, a lifetime
	// cell): focus goes to the choice that is left - or, when the choice went
	// with the link (sharing off, or the right withdrawn), to the primary.
	async function backToHousehold() {
		share = null;
		audience = 'household';
		await tick();
		(
			document.getElementById(`${uid}-household-radio`) ?? document.getElementById(`${uid}-primary`)
		)?.focus();
	}

	async function create() {
		if (creating) return;
		creating = true;
		try {
			share = await createPublicShare(recipe.id, pending);
		} catch (error) {
			// A second tab (or a double tap) got there first: the problem
			// carries that link, so this shows it instead of failing.
			const existing =
				error instanceof ApiError && error.status === 409
					? (error.body.share as PublicShare | undefined)
					: undefined;
			if (existing) share = existing;
			else refused(error, m.public_share_create_error());
		} finally {
			creating = false;
		}
	}

	// Before the link exists a cell only picks; after, every cell restarts
	// the link from now, the current one too.
	async function pick(days: ShareLifetime) {
		if (!share) {
			pending = days;
			return;
		}
		if (retiming) return;
		retiming = true;
		try {
			share = await retimeShare(share.id, days);
		} catch (error) {
			if (error instanceof ApiError && error.status === 404) {
				// Revoked meanwhile, in another tab or from Shared links.
				toast.error(m.share_retime_error());
				await backToHousehold();
			} else {
				// Sharing switched off or the right withdrawn meanwhile: the
				// link is paused now, so nothing hands it on or re-times it.
				if (error instanceof ApiError && error.status === 403 && share) {
					share = { ...share, status: 'paused' };
				}
				refused(error, m.share_retime_error());
			}
		} finally {
			retiming = false;
		}
	}

	async function revoke() {
		if (!share) return;
		try {
			await revokeShare(share.id);
		} catch (error) {
			// A 404 is a link already revoked, in another tab or from Shared
			// links: gone either way, so the sheet lets go of it too.
			if (!(error instanceof ApiError && error.status === 404)) {
				toast.error(m.public_share_revoke_error());
				return;
			}
		}
		toast.success(m.public_share_revoked());
		await backToHousehold();
	}

	// The phone's preview gives way before the controls do: step by step
	// the chip goes (1), the photo flattens (2), the photo goes (3), the
	// ticket shrinks (4, `shrink`, down to 60 %), and only then does the
	// region scroll. The ticket itself never goes: on an iPhone in Safari
	// the last step used to drop it and leave its room empty. Each step is
	// laid out and measured at once; the first that fits stays.
	function fitRegion(region: HTMLElement) {
		// Fit again when the content changes, not only on a resize.
		void everyone;
		void offered;
		void share;
		void recipe;
		// At most a pixel over, for rounding: the region's last pixels are the
		// choice's bottom edge, and a 6px allowance for the tilted ticket
		// clipped it.
		const over = () => region.scrollHeight - region.clientHeight - 1;
		const steps = () => {
			for (let step = 0; step <= 3; step++) {
				fit = step;
				flushSync();
				if (over() <= 0) return;
			}
			const ticket = region.querySelector<HTMLElement>('[data-ticket]');
			if (!ticket) return;
			const height = ticket.offsetHeight;
			fit = 4;
			// 2px more than measured: the zoomed box rounds to whole pixels.
			shrink = Math.max(0.6, (height - over() - 2) / height);
			flushSync();
		};
		const measure = () => {
			shrink = 1;
			steps();
		};
		// In the next frame, not in the observer's callback: a fit there
		// resizes the region again, a resize the observer cannot deliver in
		// the same frame ("ResizeObserver loop completed with undelivered
		// notifications").
		let frame = 0;
		const observer = new ResizeObserver(() => {
			cancelAnimationFrame(frame);
			frame = requestAnimationFrame(measure);
		});
		// The region itself keeps its size when only its content changes: a
		// web font arriving rewraps the title and the rows without resizing
		// the region, and the step measured with the fallback font stayed.
		// So its children are watched too, and the fonts' arrival.
		observer.observe(region);
		for (const child of region.children) observer.observe(child);
		let live = true;
		void document.fonts.ready.then(() => {
			if (live) frame = requestAnimationFrame(measure);
		});
		return () => {
			live = false;
			observer.disconnect();
			cancelAnimationFrame(frame);
		};
	}
</script>

<!-- The ticket is hidden from screen readers; the row and the chip say
     its audience and time, this its recipe and address. -->
{#snippet spoken()}
	<p class="sr-only">{recipe.title}, {shownAddress()}</p>
{/snippet}

{#snippet preview()}
	<!-- `zoom`, not a transform: the box shrinks with it, so what it frees
	     is room the choice below can use. -->
	<div data-ticket class="max-w-full" style:zoom={shrink < 1 ? shrink : undefined}>
		<ShareTicket
			{recipe}
			address={shownAddress()}
			{face}
			photo={fit >= 3 ? 'none' : fit >= 2 ? 'flat' : 'full'}
		/>
	</div>
	{#if fit === 0}
		{#if !everyone}
			<span
				class="inline-flex h-6.5 items-center gap-1.5 rounded-pill border border-border bg-surface px-2.5 text-micro font-semibold text-text-muted"
			>
				<Lock class="size-3.5" aria-hidden="true" />
				{m.share_chip_household()}
			</span>
		{:else if link.status === 'active'}
			<span
				class="inline-flex h-6.5 items-center gap-1.5 rounded-pill bg-success-soft px-2.5 text-micro font-semibold text-success-foreground"
			>
				<Globe class="size-3.5" aria-hidden="true" />
				{link.expiresAt
					? m.share_chip_public_until({ date: shortDate(link.expiresAt) })
					: m.share_chip_public_open()}
			</span>
		{/if}
	{/if}
{/snippet}

{#snippet row(value: Audience, title: string, Icon: typeof House, hint: Snippet)}
	<RadioGroup.Item
		{value}
		id="{uid}-{value}-radio"
		aria-labelledby="{uid}-{value}"
		aria-describedby="{uid}-{value}-hint"
		class="group flex w-full items-center gap-3 px-3.5 py-[9px] text-left text-body/[1.3] text-text focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
	>
		<span
			class="size-5 shrink-0 rounded-pill border-2 border-handle transition-[border] group-data-[state=checked]:border-6 group-data-[state=checked]:border-primary"
		></span>
		<span class="min-w-0 flex-1">
			<span id="{uid}-{value}" class="block font-semibold">{title}</span>
			<span id="{uid}-{value}-hint" class="mt-0.5 block text-caption text-pretty text-text-muted">
				{@render hint()}
			</span>
		</span>
		<Icon class="size-4.5 shrink-0 text-text-muted" aria-hidden="true" />
	</RadioGroup.Item>
{/snippet}

{#snippet householdHint()}
	{m.share_household_hint()}
{/snippet}

{#snippet everyoneHint()}
	{#if !share}
		{m.share_public_hint_new()}
	{:else if share.status === 'paused'}
		{m.detail_public_share_paused()}
	{:else if share.status === 'limited'}
		{m.detail_public_share_limited()}
		<span class="block">{m.share_public_limited_hint()}</span>
	{:else if share.expiresAt}
		{m.share_public_hint_until({ date: shortDate(share.expiresAt) })}
	{:else}
		{m.share_public_hint_open()}
	{/if}
{/snippet}

<!-- The chosen audience's own detail: who opens a household link, and how
     long a public one lasts. Each has a footer behind a perforation: the
     household's leads to People, the public one's to every shared link
     and to Revoke. Both are as tall as each other, so a pick slides the
     public row by that height and the card around them keeps its size. -->
{#snippet householdDetail()}
	<div class="flex flex-col gap-2.5 px-3.5 pt-px pb-0.5">
		<p
			class="flex min-h-13 items-center gap-3 rounded-md bg-background px-3 py-1.5 text-caption/[1.3] text-pretty text-text-muted"
		>
			<Lock class="size-4.5 shrink-0" aria-hidden="true" />
			{m.share_household_detail()}
		</p>
		<div class={footer}>
			<a href={resolve('/settings/users')} class="{footerLink} text-text-muted">
				<Users class="size-3.5" aria-hidden="true" />
				{m.settings_nav_users()}
				<ChevronRight class="-ml-0.75 size-3.5" aria-hidden="true" />
			</a>
		</div>
	</div>
{/snippet}

{#snippet publicDetail()}
	<div class="flex flex-col gap-2.5 px-3.5 pt-px pb-0.5">
		<LifetimePicker
			{options}
			value={lifetime}
			mode={share ? 'retime' : 'choose'}
			disabled={share?.status === 'paused'}
			onpick={pick}
		/>
		<div class={footer}>
			<a href={resolve('/settings/shares')} class="{footerLink} text-text-muted">
				<Tickets class="size-3.5" aria-hidden="true" />
				{m.settings_nav_shares()}
				<ChevronRight class="-ml-0.75 size-3.5" aria-hidden="true" />
			</a>
			{#if share}
				<button
					type="button"
					onclick={() => (revokeOpen = true)}
					class="{footerLink} text-destructive"
				>
					<TicketX class="size-3.5" aria-hidden="true" />
					{m.public_share_revoke()}
				</button>
			{/if}
		</div>
	</div>
{/snippet}

<!-- Who the link opens for; the chosen row unfolds its detail. -->
{#snippet choice()}
	<RadioGroup.Root
		value={audience}
		onValueChange={(next) => (audience = next as Audience)}
		aria-label={m.share_audience_label()}
		class="overflow-hidden rounded-lg border border-border bg-surface"
	>
		<div class={everyone ? '' : 'bg-surface-elevated'}>
			{@render row('household', m.share_household(), House, householdHint)}
			{#if !everyone}
				<div transition:slide={{ duration: unfold }}>{@render householdDetail()}</div>
			{/if}
		</div>
		<div class="border-t border-border {everyone ? 'bg-surface-elevated' : ''}">
			{@render row('everyone', m.share_public(), Globe, everyoneHint)}
			{#if everyone}
				<div transition:slide={{ duration: unfold }}>{@render publicDetail()}</div>
			{/if}
		</div>
	</RadioGroup.Root>
{/snippet}

{#snippet scissors()}
	<div class="relative border-t-2 border-dashed border-handle" aria-hidden="true">
		<Scissors
			class="absolute -top-2.5 left-5 size-5 -rotate-90 bg-background px-0.5 text-text-muted"
		/>
	</div>
{/snippet}

{#if phone}
	<!-- Shares the sheet's tab cell with the Image tab. The region above the
	     buttons holds the preview, the perforation and the choice, from the
	     top; it fills the cell and gives way through the fit steps. The
	     buttons stay outside it. -->
	<div class="col-start-1 row-start-1 flex min-h-0 flex-col {shown ? '' : 'invisible'}">
		<div
			{@attach fitRegion}
			data-fit={fit || ''}
			class="flex min-h-0 flex-auto flex-col overflow-y-auto overscroll-contain"
		>
			{@render spoken()}
			<div class="flex flex-col items-center gap-3.5 px-5 pt-2.5 {offered ? '' : 'pb-2.5'}">
				{@render preview()}
			</div>
			{#if offered}
				<!-- The tilted ticket's corner reaches 6px below its box, hence the
				     line's distance. Its scissors hang 10px over the line: the room
				     around it is the region's. -->
				<div class="pt-4">{@render scissors()}</div>
				<div data-choice class="px-5 pt-3.5">{@render choice()}</div>
			{/if}
		</div>
		{#if !offered}
			{@render scissors()}
		{/if}
		<div class="flex gap-2.5 px-5 pt-4 pb-[max(1.25rem,env(safe-area-inset-bottom))]">
			<!-- The padding gives way first. "Öffentlichen Link erstellen", the one
			     label that still does not fit at 150 % zoom, wraps and the button
			     grows, never past the edge; the short ones keep one line. The
			     primary says what this browser will
			     do: share where it has a share sheet, else copy - and the copy
			     button beside it only while the primary does not copy already. -->
			<Button
				id="{uid}-primary"
				size="lg"
				class="flex-1 px-3! shadow-cta {creatable
					? 'h-auto! min-h-12 py-2 text-center text-balance max-sm:min-h-14'
					: 'whitespace-nowrap'}"
				busy={creating}
				disabled={!creatable && !sendable}
				onclick={creatable
					? create
					: canShareLink()
						? () => shareLink(recipe.title, address())
						: () => copyLink(address())}
			>
				{#if creatable}
					<TicketPlus class="size-4" aria-hidden="true" />
					{m.public_share_create()}
				{:else if canShareLink()}
					<Share class="size-4" aria-hidden="true" />
					{m.share_share_link()}
				{:else}
					<Copy class="size-4" aria-hidden="true" />
					{m.detail_copy_link()}
				{/if}
			</Button>
			{#if !creatable && canShareLink()}
				<Button
					variant="secondary"
					size="lg"
					class="aspect-square shrink-0 px-0!"
					label={m.detail_copy_link()}
					disabled={!sendable}
					onclick={() => copyLink(address())}
				>
					<Copy class="size-5" aria-hidden="true" />
				</Button>
			{/if}
		</div>
	</div>
{:else if shown}
	<div class="-my-7 -ml-7 flex min-h-[480px] flex-col bg-background">
		<div class="px-5.5 pt-7 pb-1.5">{@render switcher()}</div>
		<div class="flex flex-1 flex-col items-center justify-center gap-3.5 px-5 pb-7">
			{@render preview()}
			{@render spoken()}
		</div>
	</div>
	<div class="flex flex-col py-1">
		{@render heading()}
		<p class="mt-1 text-body-sm text-text-muted">{m.share_link_dialog_hint()}</p>
		{#if offered}
			<div data-choice class="mt-5">{@render choice()}</div>
		{/if}
		<div class="mt-5 flex gap-1.5">
			<Button
				id="{uid}-primary"
				class="flex-1"
				busy={creating}
				disabled={!creatable && !sendable}
				onclick={creatable ? create : () => copyLink(address())}
			>
				{#if creatable}
					<TicketPlus class="size-4" aria-hidden="true" />
					{m.public_share_create()}
				{:else}
					<Copy class="size-4" aria-hidden="true" />
					{m.detail_copy_link()}
				{/if}
			</Button>
			{#if !creatable && canShareLink()}
				<Button
					variant="secondary"
					class="flex-1"
					disabled={!sendable}
					onclick={() => shareLink(recipe.title, address())}
				>
					<Share class="size-4" aria-hidden="true" />
					{m.share_share()}
				</Button>
			{/if}
		</div>
	</div>
{/if}

<ConfirmDialog
	bind:open={revokeOpen}
	title={m.public_share_revoke_title()}
	text={m.public_share_revoke_text()}
	confirmLabel={m.public_share_revoke()}
	destructive
	onconfirm={revoke}
/>
