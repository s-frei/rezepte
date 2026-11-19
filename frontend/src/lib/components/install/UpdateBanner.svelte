<script lang="ts">
	import ArrowRight from '@lucide/svelte/icons/arrow-right';
	import ArrowUpRight from '@lucide/svelte/icons/arrow-up-right';
	import X from '@lucide/svelte/icons/x';
	import { session } from '#lib/auth.svelte.js';
	import { UPDATES_GUIDE_URL } from '#lib/docs.js';
	import { m } from '#lib/paraglide/messages.js';
	import { getLocale } from '#lib/paraglide/runtime.js';
	import { noteVisible } from '#lib/update.js';
	import { dismissUpdate, update } from '#lib/update.svelte.js';
	import { collapse } from './collapse.js';

	const visible = $derived(
		session.user?.role === 'superadmin' && noteVisible(update.status, update.dismissed)
	);

	// The day the release came out, in the reader's language; with no date,
	// or one the browser cannot read, the caption is left out.
	const since = $derived.by(() => {
		const date = new Date(update.status?.latest?.publishedAt ?? '');
		if (Number.isNaN(date.getTime())) return '';
		return new Intl.DateTimeFormat(getLocale(), { day: 'numeric', month: 'long' }).format(date);
	});

	let folding = false;

	function dismiss() {
		folding = true;
		dismissUpdate();
	}

	const pill =
		'inline-flex h-11 items-center justify-center gap-2 rounded-pill text-body-sm font-semibold transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary active:scale-[.98] @min-[40rem]:px-5';
</script>

<!--
	The owner's note that a newer Rezepte is out, in the install tip's frame
	and on every width: the installed version, the new one, and two links out
	to its release notes and the update guide. × hides it until a newer
	version comes; the menu's version band stays. Both links leave the SPA,
	so `resolve()` does not apply and the navigation rule is lifted for them.
-->
{#if visible && update.status?.latest}
	{@const latest = update.status.latest}
	<!-- Two containers: the outer one sizes the note's padding, the note
	     itself its rows. -->
	<div out:collapse={() => folding} class="@container mb-5">
		<div class="@container rounded-2xl bg-surface px-4 pt-3 pb-4 shadow-card @min-[40rem]:px-5">
			<div class="flex items-center gap-2.5">
				<span class="font-display text-body-sm font-medium text-primary italic">
					{m.update_note_label()}
				</span>
				<span
					class="min-w-4 flex-1 -translate-y-0.5 border-b-2 border-dotted border-handle"
					aria-hidden="true"
				></span>
				<!-- 44px target, pulled out with negative margins so the look stays. -->
				<button
					type="button"
					onclick={dismiss}
					aria-label={m.update_note_dismiss()}
					title={m.update_note_dismiss()}
					class="-my-1 -mr-3 flex size-11 shrink-0 items-center justify-center text-text-muted transition hover:text-text focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
				>
					<X class="size-4" aria-hidden="true" />
				</button>
			</div>
			<!-- Under 40rem the actions sit below the versions, side by side
		     while each gets 7rem; from 40rem both share one row. -->
			<div
				class="mt-0.5 grid gap-4 @min-[40rem]:grid-cols-[1fr_auto] @min-[40rem]:items-end @min-[40rem]:gap-x-8"
			>
				<div class="min-w-0">
					<p class="font-display text-heading font-medium text-balance">
						{m.update_note_title()}
					</p>
					<div
						role="group"
						aria-label={m.update_versions_label({
							from: update.status.current,
							to: latest.version
						})}
						class="mt-2.5 flex items-start gap-3"
					>
						<div class="grid gap-0.5">
							<span
								class="font-display text-heading leading-[1.1] font-medium text-text-muted lining-nums tabular-nums"
							>
								{update.status.current}
							</span>
							<span class="text-caption text-text-muted">{m.update_note_installed()}</span>
						</div>
						<ArrowRight class="mt-1 size-4.5 self-start text-handle" aria-hidden="true" />
						<div class="grid gap-0.5">
							<span
								class="font-display text-heading leading-[1.1] font-medium text-primary lining-nums tabular-nums"
							>
								{latest.version}
							</span>
							{#if since}
								<span class="text-caption text-text-muted"
									>{m.update_note_since({ date: since })}</span
								>
							{/if}
						</div>
					</div>
				</div>
				<div
					class="grid grid-cols-[repeat(auto-fit,minmax(7rem,1fr))] gap-2.5 @min-[40rem]:grid-cols-[auto_auto]"
				>
					<!-- eslint-disable svelte/no-navigation-without-resolve -->
					<a
						href={latest.changelogUrl}
						target="_blank"
						rel="noopener external"
						class="{pill} bg-primary hover-tint text-primary-foreground"
					>
						{m.update_note_whats_new()}
						<ArrowUpRight class="size-4" aria-hidden="true" />
					</a>
					<a
						href={UPDATES_GUIDE_URL}
						target="_blank"
						rel="noopener external"
						class="{pill} border border-border bg-surface-elevated hover:bg-background"
					>
						{m.update_note_instructions()}
						<ArrowUpRight class="size-4" aria-hidden="true" />
					</a>
					<!-- eslint-enable svelte/no-navigation-without-resolve -->
				</div>
			</div>
		</div>
	</div>
{/if}
