<script lang="ts">
	import Code from '@lucide/svelte/icons/code';
	import Globe from '@lucide/svelte/icons/globe';
	import LogOut from '@lucide/svelte/icons/log-out';
	import UserRound from '@lucide/svelte/icons/user-round';
	import UsersRound from '@lucide/svelte/icons/users-round';
	import { Dialog } from 'bits-ui';
	import { afterNavigate } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { session } from '$lib/auth.svelte';
	import BottomSheet from '$lib/components/ui/BottomSheet.svelte';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/roles';
	import { signOut } from '$lib/sign-out';
	import { userColorClasses } from '$lib/user/color';

	let { open = $bindable(false) }: { open?: boolean } = $props();

	// Back and forward navigate without a tap in the sheet; it closes with
	// them, as the command palette does.
	afterNavigate(() => {
		open = false;
	});

	const user = $derived(session.user);
	const initial = $derived(user?.displayName.charAt(0).toUpperCase() ?? '');

	// The settings pages in the order `SettingsLayout` lists them.
	const pages = [
		{ id: 'profile', href: resolve('/settings'), label: m.settings_nav_profile(), icon: UserRound },
		{ id: 'api', href: resolve('/settings/api'), label: m.settings_nav_api(), icon: Code },
		{
			id: 'shares',
			href: resolve('/settings/shares'),
			label: m.settings_nav_shares(),
			icon: Globe
		},
		{
			id: 'people',
			href: resolve('/settings/users'),
			label: m.settings_nav_users(),
			icon: UsersRound
		}
	];

	const row =
		'flex min-h-12 w-full items-center gap-3 rounded-md px-3 py-3 text-left text-body font-medium transition hover:bg-surface focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary';
</script>

<!--
	Who is signed in, and the pages that belong to them: a phone's way to the
	settings and to signing out. The same sheet the contents and the people
	list open, headed by the person rather than by a word, so the bar's "You"
	opens onto exactly that.
-->
{#if user}
	<BottomSheet bind:open closeLabel={m.common_close()}>
		<div class="min-h-0 flex-1 overflow-y-auto px-5 pb-8">
			<div class="flex items-center gap-4 border-b border-border pb-5">
				<span
					aria-hidden="true"
					class="flex size-14 shrink-0 items-center justify-center rounded-full initial-centered font-display text-heading font-semibold {userColorClasses(
						user.color
					)}"
				>
					{initial}
				</span>
				<div class="min-w-0">
					<Dialog.Title class="truncate font-display text-heading font-medium">
						{user.displayName}
					</Dialog.Title>
					<p class="truncate text-caption text-text-muted">
						{m.nav_you_signed_in({ role: roleLabel(user.role), username: user.username })}
					</p>
				</div>
			</div>
			<div class="-mx-3 pt-2">
				{#each pages as entry (entry.id)}
					{@const Icon = entry.icon}
					<!-- `href` is a `ResolvedPathname`: `pages` resolved it. -->
					<!-- eslint-disable-next-line svelte/no-navigation-without-resolve -->
					<a href={entry.href} onclick={() => (open = false)} class={row}>
						<Icon class="size-5 shrink-0 text-text-muted" aria-hidden="true" />
						{entry.label}
					</a>
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
