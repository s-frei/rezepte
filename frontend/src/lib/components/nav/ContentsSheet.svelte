<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { prefersReducedMotion } from 'svelte/motion';
	import { fade, fly } from 'svelte/transition';
	import { m } from '$lib/paraglide/messages';
	import type { ContentsAction, ContentsEntry } from './contents';

	let {
		open = $bindable(false),
		entries,
		current,
		actions = [],
		onselect
	}: {
		open?: boolean;
		entries: ContentsEntry[];
		/** Id of the entry to mark as the current one. */
		current: string;
		/** Rows that are not entries (sign out), under a rule at the foot. */
		actions?: ContentsAction[];
		/** Called for an entry without an `href`, after the sheet has closed. */
		onselect?: (id: string) => void;
	} = $props();

	const duration = $derived(prefersReducedMotion.current ? 0 : 250);
	const fadeDuration = $derived(prefersReducedMotion.current ? 0 : 200);

	function pick(entry: ContentsEntry) {
		open = false;
		if (entry.href === undefined) {
			onselect?.(entry.id);
		}
	}

	const row =
		'flex min-h-12 w-full items-baseline gap-2 rounded-md px-3 py-3 text-left transition focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary';
</script>

<!--
	The contents page of a cookbook, as a bottom sheet: every entry on its own
	line, its name in the display serif, a dotted leader, and on the right
	what a book would print as the page number - here a count, a title or a
	setting. The one element of the phone's navigation allowed to be
	memorable; the running head that opens it stays quiet.
-->
<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
						transition:fade={{ duration: fadeDuration }}
					></div>
				{/if}
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content forceMount preventScroll={false}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-x-0 bottom-0 z-50 mx-auto flex max-h-[80dvh] w-full max-w-[640px] flex-col rounded-t-3xl bg-background shadow-sheet outline-none"
						transition:fly={{ duration, y: 200 }}
					>
						<Dialog.Close
							aria-label={m.nav_contents_close()}
							class="flex w-full shrink-0 justify-center pt-2 pb-3 focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary"
						>
							<span class="h-1 w-9 rounded-pill bg-handle" aria-hidden="true"></span>
						</Dialog.Close>
						<Dialog.Title class="px-5 pb-2 font-display text-heading-lg font-medium italic">
							{m.nav_contents_title()}
						</Dialog.Title>
						<div class="min-h-0 flex-1 overflow-y-auto px-2 pb-8">
							<ol>
								{#each entries as entry (entry.id)}
									{@const isCurrent = entry.id === current}
									<li>
										{#snippet content()}
											<span
												class="shrink-0 font-display text-body-lg {isCurrent
													? 'font-semibold text-primary'
													: 'font-medium text-text'}"
											>
												{entry.label}
											</span>
											{#if entry.summary !== undefined}
												<span
													aria-hidden="true"
													class="min-w-6 flex-1 border-b-2 border-dotted {isCurrent
														? 'border-primary/40'
														: 'border-handle'}"
												></span>
												<span
													class="max-w-[60%] min-w-0 truncate text-body-sm tabular-nums {entry.placeholder
														? 'text-text-muted italic'
														: isCurrent
															? 'font-semibold text-primary'
															: 'text-text-muted'}"
												>
													{entry.summary}
												</span>
											{/if}
										{/snippet}
										{#if entry.href !== undefined}
											<!-- `href` is a `ResolvedPathname`: the caller resolved it. -->
											<!-- eslint-disable svelte/no-navigation-without-resolve -->
											<a
												href={entry.href}
												aria-current={isCurrent ? 'page' : undefined}
												onclick={() => pick(entry)}
												class="{row} {isCurrent ? 'bg-surface' : 'hover:bg-surface'}"
											>
												{@render content()}
											</a>
											<!-- eslint-enable svelte/no-navigation-without-resolve -->
										{:else}
											<button
												type="button"
												aria-current={isCurrent ? 'true' : undefined}
												onclick={() => pick(entry)}
												class="{row} {isCurrent ? 'bg-surface' : 'hover:bg-surface'}"
											>
												{@render content()}
											</button>
										{/if}
									</li>
								{/each}
							</ol>
							{#if actions.length > 0}
								<div class="mx-3 my-2 h-px bg-border" aria-hidden="true"></div>
								{#each actions as action (action.id)}
									<button
										type="button"
										onclick={() => {
											open = false;
											action.onselect();
										}}
										class="{row} hover:bg-surface {action.tone === 'destructive'
											? 'text-destructive'
											: 'text-text'} text-body"
									>
										{action.label}
									</button>
								{/each}
							{/if}
						</div>
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
