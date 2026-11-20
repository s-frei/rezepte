<script lang="ts">
	import CalendarCheck from '@lucide/svelte/icons/calendar-check';
	import RefreshCw from '@lucide/svelte/icons/refresh-cw';
	import X from '@lucide/svelte/icons/x';
	import { Dialog } from 'bits-ui';
	import { tick, untrack } from 'svelte';
	import { toast } from 'svelte-sonner';
	import { isSignedOut } from '#lib/api/client.js';
	import { createCalendarFeed, deleteCalendarFeed, getCalendarFeed } from '#lib/api/mealplan.js';
	import CopyIcon from '#lib/components/icons/CopyIcon.svelte';
	import Button from '#lib/components/ui/Button.svelte';
	import ConfirmDialog from '#lib/components/ui/ConfirmDialog.svelte';
	import ResponsiveDialog from '#lib/components/ui/ResponsiveDialog.svelte';
	import { copyText } from '#lib/recipe/share.svelte.js';
	import { formatDate } from '#lib/recipe/format.js';
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * "Im Kalender abonnieren": the reader's one secret feed over every plan
	 * they are in, the same from every plan's ⋯. Only its hash is stored, so
	 * the address shows once, right after it is made; later openings say since
	 * when it works and offer a new one (which revokes the old) or the end.
	 */
	let { open = $bindable(false) }: { open?: boolean } = $props();

	type State =
		| { kind: 'loading' }
		| { kind: 'failed' }
		| { kind: 'none' }
		| { kind: 'active'; since: string }
		| { kind: 'fresh'; url: string };

	let feed = $state<State>({ kind: 'loading' });
	let busy = $state<'' | 'create' | 'end'>('');
	let confirmReplace = $state(false);
	let confirmEnd = $state(false);
	let copyIcon = $state<CopyIcon>();
	/** Focus after a change, on the control that comes next (`data-calendar-focus`). */
	async function focusNext(name: 'copy' | 'create') {
		await tick();
		document.querySelector<HTMLElement>(`[data-calendar-focus="${name}"]`)?.focus();
	}

	// Read afresh on every opening; a link made before is never shown again.
	$effect(() => {
		if (!open) return;
		untrack(() => void load());
	});

	let req = 0;
	async function load() {
		const mine = ++req;
		feed = { kind: 'loading' };
		try {
			const state = await getCalendarFeed();
			if (mine !== req) return;
			feed =
				state.active && state.createdAt
					? { kind: 'active', since: state.createdAt }
					: { kind: 'none' };
		} catch (error) {
			if (mine !== req) return;
			feed = { kind: 'failed' };
			if (!isSignedOut(error)) toast.error(m.plan_calendar_error());
		}
	}

	/** Without REZEPTE_PUBLIC_URL the server answers a bare path: the page's origin completes it. */
	const absolute = (url: string) => new URL(url, window.location.origin).toString();

	async function create() {
		if (busy) return;
		busy = 'create';
		try {
			const { url } = await createCalendarFeed();
			req++;
			feed = { kind: 'fresh', url: absolute(url) };
			// The next thing to do is copying it.
			await focusNext('copy');
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.plan_calendar_error());
		} finally {
			busy = '';
		}
	}

	async function end() {
		if (busy) return;
		busy = 'end';
		try {
			await deleteCalendarFeed();
			req++;
			feed = { kind: 'none' };
			toast.success(m.plan_calendar_ended());
			await focusNext('create');
		} catch (error) {
			if (!isSignedOut(error)) toast.error(m.plan_calendar_error());
		} finally {
			busy = '';
		}
	}

	async function copy(url: string) {
		if (await copyText(url, m.plan_calendar_copied())) copyIcon?.play();
	}

	/** The same address for the calendar app's own scheme: one tap subscribes on an iPhone or a Mac. */
	const webcal = (url: string) => url.replace(/^https?:/, 'webcal:');

	const subscribed = $derived(feed.kind === 'active' || feed.kind === 'fresh');
	const label = 'text-label font-bold tracking-[0.08em] text-text-muted uppercase';
</script>

{#snippet body()}
	<Dialog.Title class="font-display text-heading font-medium">{m.plan_calendar()}</Dialog.Title>
	<Dialog.Description class="mt-2 text-body-sm text-text-muted">
		{m.plan_calendar_text()}
	</Dialog.Description>

	<div class="mt-5" aria-busy={feed.kind === 'loading'}>
		{#if feed.kind === 'loading'}
			<div class="h-14 animate-pulse rounded-md bg-surface-elevated"></div>
		{:else if feed.kind === 'failed'}
			<button
				type="button"
				onclick={load}
				class="flex min-h-11 w-full items-center justify-between rounded-md border border-border bg-surface-elevated px-4 py-1.5 text-left text-body-sm text-text-muted focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
			>
				<span class="min-w-0 text-caption leading-tight">{m.plan_calendar_error()}</span>
				<span class="shrink-0 pl-3 font-semibold whitespace-nowrap text-primary"
					>{m.add_to_list_retry()}</span
				>
			</button>
		{:else if feed.kind === 'active'}
			<div
				class="flex items-start gap-3 rounded-lg bg-success-soft px-4 py-3.5 text-success-foreground"
			>
				<CalendarCheck class="mt-0.5 size-5 shrink-0" aria-hidden="true" />
				<p class="min-w-0 text-body-sm">
					{m.plan_calendar_active({ date: formatDate(feed.since) })}
				</p>
			</div>
		{:else if feed.kind === 'fresh'}
			{@const url = feed.url}
			<p id="calendar-link-label" class="mb-2 {label}">{m.plan_calendar_link_label()}</p>
			<!-- SetupLinkDialog's value block: the address wraps instead of scrolling
			     out of view, readonly so it can still be selected by hand; on a
			     narrow phone with large text the copy button goes under it. -->
			<div class="flex flex-wrap items-start gap-2 rounded-md bg-surface-elevated p-3">
				<textarea
					readonly
					rows="3"
					value={url}
					aria-labelledby="calendar-link-label"
					aria-describedby="calendar-link-secret"
					class="[field-sizing:content] min-w-0 flex-1 basis-40 resize-none bg-transparent font-mono text-body-sm break-all outline-none"
					onclick={(event) => event.currentTarget.select()}></textarea>
				<Button
					variant="ghost"
					class="shrink-0 whitespace-nowrap max-md:h-11"
					data-calendar-focus="copy"
					onclick={() => void copy(url)}
				>
					<CopyIcon bind:this={copyIcon} class="size-4" />
					{m.tokens_reveal_copy()}
				</Button>
			</div>
			<p id="calendar-link-secret" class="mt-2 text-caption text-text-muted">
				{m.plan_calendar_secret()}
			</p>
		{/if}

		{#if subscribed}
			<!-- One row on the narrowest phone too: the icons give way below 400px. -->
			<div class="mt-4 flex flex-wrap gap-2">
				<Button
					variant="secondary"
					size="sm"
					class="max-[399px]:px-3 max-md:h-11"
					busy={busy === 'create'}
					onclick={() => (confirmReplace = true)}
				>
					<RefreshCw class="size-4 max-[399px]:hidden" aria-hidden="true" />
					{m.plan_calendar_replace()}
				</Button>
				<!-- Button's secondary look in the destructive ink, as the item sheet's remove. -->
				<button
					type="button"
					aria-busy={busy === 'end'}
					onclick={() => (confirmEnd = true)}
					class="inline-flex h-9 items-center justify-center gap-2 rounded-pill border border-border bg-surface hover-tint px-4 text-body-sm font-semibold whitespace-nowrap text-destructive transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] max-[399px]:px-3 max-md:h-11"
				>
					<X class="size-4 shrink-0 max-[399px]:hidden" aria-hidden="true" />
					{m.plan_calendar_end()}
				</button>
			</div>
		{/if}
	</div>

	{#if feed.kind !== 'loading' && feed.kind !== 'failed'}
		<section class="mt-6" aria-labelledby="calendar-how">
			<h3 id="calendar-how" class={label}>{m.plan_calendar_how()}</h3>
			<ol class="mt-2 space-y-2 text-body-sm">
				<li>
					{m.plan_calendar_iphone()}
					{#if feed.kind === 'fresh'}
						<!-- A link to another app's scheme, not a route. -->
						<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
						<a
							href={webcal(feed.url)}
							class="flex min-h-11 items-center font-semibold text-primary hover:underline focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary md:mt-0.5 md:min-h-0"
							>{m.plan_calendar_open()}</a
						>
					{/if}
				</li>
				<li>{m.plan_calendar_google()}</li>
			</ol>
		</section>
	{/if}
{/snippet}

{#snippet footer()}
	{#if feed.kind === 'none'}
		<Button variant="secondary" class="grow md:grow-0" onclick={() => (open = false)}
			>{m.common_cancel()}</Button
		>
		<Button
			class="grow md:grow-0"
			busy={busy === 'create'}
			data-calendar-focus="create"
			onclick={() => void create()}
		>
			{m.plan_calendar_create()}
		</Button>
	{:else}
		<Button class="grow md:grow-0" onclick={() => (open = false)}>{m.item_sheet_done()}</Button>
	{/if}
{/snippet}

<ResponsiveDialog bind:open closeLabel={m.common_close()} {body} {footer} />

<ConfirmDialog
	bind:open={confirmReplace}
	title={m.plan_calendar_replace_title()}
	text={m.plan_calendar_replace_text()}
	confirmLabel={m.plan_calendar_replace()}
	onconfirm={() => void create()}
/>

<ConfirmDialog
	bind:open={confirmEnd}
	title={m.plan_calendar_end_title()}
	text={m.plan_calendar_end_text()}
	confirmLabel={m.plan_calendar_end()}
	destructive
	onconfirm={() => void end()}
/>
