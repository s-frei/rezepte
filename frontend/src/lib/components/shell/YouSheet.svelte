<script lang="ts">
	import LogOut from '@lucide/svelte/icons/log-out';
	import { Dialog } from 'bits-ui';
	import { afterNavigate } from '$app/navigation';
	import { session } from '$lib/auth.svelte';
	import ThemeControl from '$lib/components/settings/ThemeControl.svelte';
	import { byGroup } from '$lib/components/nav/contents';
	import GroupHeading from '$lib/components/nav/GroupHeading.svelte';
	import BottomSheet from '$lib/components/ui/BottomSheet.svelte';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/roles';
	import { settingsPages } from '$lib/settings/pages';
	import { signOut } from '$lib/sign-out';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	// Back and forward navigate without a tap in the sheet; it closes with
	// them, as the command palette does.
	afterNavigate(({ from, to, type }) => {
		// A page rewriting its own query (the overview's filters, once its
		// list arrives) has not been left; anything else closes it.
		if (type === 'goto' && from?.url.pathname === to?.url.pathname) return;
		open = false;
	});

	const user = $derived(session.user);

	const pages = $derived(settingsPages(user?.role));

	const row =
		'flex min-h-12 w-full items-center gap-3 rounded-md px-3 py-3 text-left text-body font-medium transition hover:bg-surface focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary';
</script>

<!--
	Who is signed in, and the pages that belong to them: a phone's way to the
	settings and to signing out. The same sheet the contents and the people
	list open, headed by the person rather than by a word, so the bar's "You"
	opens onto exactly that. The person and the theme share one surface card:
	the theme is how this person sees the app, and the card's edge is what
	sets the head apart from the pages, where a rule and a card of its own
	took twice the height. The `bg-surface` is also what the control's
	`bg-background` track needs to show.
-->
{#if user}
	<BottomSheet bind:open closeLabel={m.common_close()}>
		<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
			<div class="-mx-1 rounded-xl bg-surface p-4">
				<div class="flex items-center gap-4">
					<PersonMark person={user} size="xl" />
					<div class="min-w-0">
						<Dialog.Title class="truncate font-display text-heading font-medium">
							{user.displayName}
						</Dialog.Title>
						<p class="truncate text-caption text-text-muted">
							{m.nav_you_signed_in({ role: roleLabel(user.role), username: user.username })}
						</p>
					</div>
				</div>
				<div class="mt-3.5">
					<ThemeControl hint={false} />
				</div>
			</div>
			<div class="-mx-3 pt-2">
				{#each byGroup(pages) as run, i (run.label)}
					<div role="group" aria-label={run.label}>
						<GroupHeading label={run.label} first={i === 0} class="px-3" />
						{#each run.entries as entry (entry.id)}
							{@const Icon = entry.icon}
							<!-- `href` is a `ResolvedPathname`: `pages` resolved it. -->
							<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
							<a href={entry.href} onclick={() => (open = false)} class={row}>
								<Icon class="size-5 shrink-0 text-text-muted" aria-hidden="true" />
								{entry.label}
							</a>
						{/each}
					</div>
				{/each}
				<div class="mx-3 my-2 h-px bg-border" aria-hidden="true"></div>
				<button
					type="button"
					onclick={() => {
						open = false;
						void signOut();
					}}
					class="{row} text-destructive"
				>
					<LogOut class="size-5 shrink-0" aria-hidden="true" />
					{m.logout()}
				</button>
			</div>
		</div>
	</BottomSheet>
{/if}
