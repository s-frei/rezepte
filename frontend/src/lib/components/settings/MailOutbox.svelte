<script lang="ts">
	import { isSignedOut } from '#lib/api/client.js';
	import { getOutbox, type Outbox, type SentMail } from '#lib/api/mail.js';
	import Button from '#lib/components/ui/Button.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import Skeleton from '#lib/components/ui/Skeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { entryTime, groupByDay } from '#lib/recipe/diary.js';
	import { addressSegments } from './Letterhead.svelte';

	let {
		refresh,
		quietWhenEmpty = false
	}: {
		/** Any change reloads the outbox, such as a test mail that went out. */
		refresh?: unknown;
		/** Mail is off: an empty outbox shows nothing at all. */
		quietWhenEmpty?: boolean;
	} = $props();

	/** How often the outbox looks again while a mail is on its way. */
	const POLL_MS = 15_000;
	/** Mails per page, as the service answers them (`historyPage` in package mail). */
	const PAGE = 50;

	let box = $state<Outbox | null>(null);
	let loadFailed = $state(false);
	let loadingMore = $state(false);
	let filter = $state<'failed' | 'queued' | null>(null);

	// A filter flip, a refresh and the poll can overlap: each read takes the
	// next number and only the newest is applied.
	let seq = 0;

	async function load() {
		const mine = ++seq;
		try {
			const next = await getOutbox({ status: filter ?? undefined });
			if (mine !== seq) return;
			box = next;
			loadFailed = false;
		} catch (error) {
			if (mine === seq) loadFailed = !isSignedOut(error);
		}
	}

	// An older page belongs to the list it was asked for: one that a filter
	// flip or a reload replaced meanwhile drops it.
	async function loadMore() {
		const base = box;
		const last = base?.items.at(-1);
		if (!base || !last || loadingMore) return;
		// A poll still in flight would answer with the first page alone.
		seq++;
		loadingMore = true;
		try {
			const page = await getOutbox({ before: last.id, status: filter ?? undefined });
			if (box !== base) return;
			box = { ...page, items: [...base.items, ...page.items] };
			loadFailed = false;
		} catch (error) {
			if (box === base) loadFailed = !isSignedOut(error);
		} finally {
			loadingMore = false;
		}
	}

	$effect(() => {
		void refresh;
		void filter;
		void load();
	});

	// A mail on its way settles within minutes: look again until it has.
	// Only the first page reloads, so the poll waits while older pages show.
	$effect(() => {
		if (!box?.queued || box.items.length > PAGE) return;
		const timer = setInterval(() => {
			if (!loadingMore && document.visibilityState === 'visible') void load();
		}, POLL_MS);
		return () => clearInterval(timer);
	});

	const days = $derived(box ? groupByDay(box.items, new Date(), { weekday: false }) : []);

	let heading = $state<HTMLElement>();

	function toggle(status: 'failed' | 'queued') {
		// Unpressing a chip whose count went to 0 removes it, and focus with
		// it: hand focus to the outbox's heading first.
		if (filter === status && box?.[status] === 0) heading?.focus({ preventScroll: true });
		filter = filter === status ? null : status;
	}

	/** Rezepte's own two reasons come as codes; a mail server's answer as it said it. */
	function reason(error: string): { text: string; own: boolean } {
		if (error === 'interrupted') return { text: m.settings_mail_outbox_interrupted(), own: true };
		if (error === 'disabled') return { text: m.settings_mail_outbox_disabled(), own: true };
		return { text: error, own: false };
	}

	function replyLabel(mail: SentMail): string {
		if (mail.status === 'failed') return m.settings_mail_outbox_reply();
		if (mail.status === 'queued') return m.settings_mail_outbox_last_attempt();
		return m.settings_mail_outbox_previous_attempt();
	}

	const [toBefore, toAfter] = m.settings_mail_outbox_to({ address: '\u0000' }).split('\u0000');

	const chip =
		'relative inline-flex h-7 items-center gap-1.5 rounded-pill border border-border bg-surface-elevated px-2.5 text-caption font-semibold whitespace-nowrap';
	const pressable =
		'transition after:absolute after:inset-x-0 after:-inset-y-2.5 hover-tint focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary aria-pressed:border-inverse aria-pressed:bg-inverse aria-pressed:text-inverse-foreground';
</script>

{#snippet address(value: string)}
	{#each addressSegments(value) as segment, i (i)}{segment}<wbr />{/each}
{/snippet}

<!-- The outbox: what went out, set apart from the settings above as a
     recessed tray - the configuration is the sheet on top, the outbox the
     tray beneath it. Delivered is the quiet default; only a mail on its
     way or not delivered carries a badge, and a retry that got through
     keeps what the earlier attempt ran into. -->
{#if loadFailed && !box}
	<div class="mt-5 border-t border-border pt-5">
		<p class="text-body-sm text-text-muted">{m.settings_mail_outbox_load_error()}</p>
	</div>
{:else if !box}
	<div class="mt-5 border-t border-border pt-5"><Skeleton class="h-40 rounded-xl" /></div>
{:else if box.total > 0 || !quietWhenEmpty}
	<section class="mt-5 border-t border-border pt-5" aria-labelledby="mail-outbox">
		<h3
			id="mail-outbox"
			bind:this={heading}
			tabindex="-1"
			class="font-display text-card font-medium focus:outline-none"
		>
			{m.settings_mail_outbox_title()}
		</h3>
		<p class="mt-1 max-w-[60ch] text-caption text-text-muted">{m.settings_mail_outbox_intro()}</p>

		{#if box.total > 0}
			<div class="mt-3 flex flex-wrap gap-2">
				{#if box.failed > 0 || filter === 'failed'}
					<button
						type="button"
						class="{chip} {pressable}"
						aria-pressed={filter === 'failed'}
						onclick={() => toggle('failed')}
					>
						<span aria-hidden="true" class="size-2 rounded-full bg-destructive"></span>
						{m.settings_mail_outbox_failed({ count: box.failed })}
					</button>
				{/if}
				{#if box.queued > 0 || filter === 'queued'}
					<button
						type="button"
						class="{chip} {pressable}"
						aria-pressed={filter === 'queued'}
						onclick={() => toggle('queued')}
					>
						<span aria-hidden="true" class="size-2 rounded-full bg-primary"></span>
						{m.settings_mail_outbox_queued({ count: box.queued })}
					</button>
				{/if}
				{#if box.failed === 0 && box.queued === 0 && filter === null}
					<span class={chip}>
						<span aria-hidden="true" class="size-2 rounded-full bg-success"></span>
						{m.settings_mail_outbox_all_delivered()}
					</span>
				{/if}
			</div>
		{/if}

		{#if box.items.length === 0}
			<p class="mt-4 text-body-sm text-text-muted">
				{filter ? m.settings_mail_outbox_problems_empty() : m.settings_mail_outbox_empty()}
			</p>
		{:else}
			<div
				class="mt-3.5 rounded-xl bg-background px-4 pt-1.5 pb-3.5 inset-shadow-hole max-md:-mx-2 max-md:px-3"
			>
				{#each days as day (day.key)}
					<div class="mt-4.5 flex items-baseline gap-2.5 first:mt-3">
						<h4 class="shrink-0 font-display text-body-lg whitespace-nowrap text-primary italic">
							{day.label}
						</h4>
						<span
							aria-hidden="true"
							class="min-w-6 flex-1 -translate-y-1 border-b-2 border-dotted border-border"
						></span>
						<!-- A day that runs on into the next page has no count yet. -->
						{#if !(box.more && day === days.at(-1))}
							<span class="text-caption whitespace-nowrap text-text-muted">
								{day.entries.length === 1
									? m.settings_mail_outbox_day_count_one()
									: m.settings_mail_outbox_day_count({ count: day.entries.length })}
							</span>
						{/if}
					</div>
					<ul>
						{#each day.entries as mail (mail.id)}
							<li
								class="grid grid-cols-[minmax(0,1fr)_auto] gap-x-3 gap-y-0.75 py-2.75 not-first:border-t not-first:border-border"
							>
								<span class="min-w-0 leading-snug font-semibold wrap-break-word hyphens-auto">
									{mail.subject}
								</span>
								<time class="pt-0.5 text-caption text-text-muted" datetime={mail.createdAt}>
									{entryTime(mail.createdAt)}
								</time>
								<span
									class="col-span-2 flex flex-wrap items-center gap-x-2.5 gap-y-1 text-caption text-text-muted"
								>
									<!-- Who the address belongs to, as the API tokens name their
									     issuer; a stranger's address (a test mail) stands alone. -->
									<span class="flex min-w-0 flex-wrap items-center gap-x-1.5 gap-y-1"
										>{toBefore}{#each mail.people as person (person.id)}<span
												class="inline-flex items-center gap-1 font-semibold text-text"
												><PersonMark size="xs" {person} />{person.displayName}</span
											>{/each}<span class="min-w-0 wrap-break-word">{@render address(mail.to)}</span
										>{toAfter}</span
									>
									{#if mail.status === 'queued'}
										<span
											class="rounded-pill bg-accent px-2 text-label font-bold whitespace-nowrap text-accent-foreground uppercase"
										>
											{m.settings_mail_outbox_badge_queued()}
										</span>
										{#if mail.nextAttemptAt}
											<time datetime={mail.nextAttemptAt}
												>{m.settings_mail_outbox_next_attempt({
													time: entryTime(mail.nextAttemptAt)
												})}</time
											>
										{/if}
									{:else if mail.status === 'failed'}
										<span
											class="rounded-pill bg-destructive-soft px-2 text-label font-bold whitespace-nowrap text-destructive uppercase"
										>
											{m.settings_mail_outbox_badge_failed()}
										</span>
									{:else if mail.attempts > 1}
										<span>{m.settings_mail_outbox_retried({ count: mail.attempts })}</span>
									{/if}
								</span>
								{#if mail.error}
									{@const why = reason(mail.error)}
									<p
										class="col-span-2 mt-1.5 rounded-md bg-surface-elevated px-2.5 py-2 text-caption {mail.status ===
										'failed'
											? 'text-text'
											: 'text-text-muted'}"
									>
										{#if !why.own}
											<span class="block text-text-muted">{replyLabel(mail)}</span>
										{/if}
										<span class={why.own ? '' : 'font-mono wrap-anywhere'}>{why.text}</span>
									</p>
								{/if}
							</li>
						{/each}
					</ul>
				{/each}
			</div>
			{#if box.more}
				<div class="mt-3 flex flex-wrap items-center gap-x-3 gap-y-2">
					<Button variant="secondary" busy={loadingMore} onclick={() => void loadMore()}>
						{m.settings_mail_outbox_more()}
					</Button>
				</div>
			{/if}
		{/if}
		<!-- Always there, empty while nothing failed: a live region inserted
		     with its text is often not announced. A failed flip keeps the
		     list it had, and this says so. -->
		<p class={loadFailed ? 'mt-3 text-caption text-text-muted' : ''} role="status">
			{loadFailed ? m.settings_mail_outbox_load_error() : ''}
		</p>
	</section>
{/if}
