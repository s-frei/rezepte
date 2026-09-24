<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import { session } from '$lib/auth.svelte';
	import ContentsSheet from '$lib/components/nav/ContentsSheet.svelte';
	import RunningHead from '$lib/components/nav/RunningHead.svelte';
	import { m } from '$lib/paraglide/messages';
	import { isAdminRole } from '$lib/roles';
	import { shell } from '$lib/shell.svelte';
	import { signOut } from '$lib/sign-out';

	let { active, children }: { active: 'profile' | 'users' | 'api'; children: Snippet } = $props();

	const isAdmin = $derived(isAdminRole(session.user?.role));

	// Same top-bar contract as the editor: breadcrumb instead of the default
	// action buttons, reset when leaving.
	$effect(() => {
		shell.breadcrumb = m.settings_title();
		shell.actions = noActions;
		return () => {
			shell.breadcrumb = undefined;
			shell.actions = undefined;
		};
	});

	// The pages of the settings area, in the order both navs list them. The
	// members page exists for admins only.
	const pages = $derived([
		{ id: 'profile', href: resolve('/settings'), label: m.settings_nav_profile() },
		{ id: 'api', href: resolve('/settings/api'), label: m.settings_nav_api() },
		...(isAdmin
			? [{ id: 'users', href: resolve('/settings/users'), label: m.settings_nav_users() }]
			: [])
	]);

	let contentsOpen = $state(false);
	// Signing out is not a page, so on a phone it is an action at the foot of
	// the contents sheet, set apart the way the sidebar sets it apart.
	const actions = [
		{
			id: 'sign-out',
			label: m.logout(),
			tone: 'destructive' as const,
			onselect: () => void signOut()
		}
	];
</script>

{#snippet noActions()}{/snippet}

<div class="pt-6 pb-8 md:pt-10">
	<h1 class="font-display text-display-sm font-medium md:text-display-lg">{m.settings_title()}</h1>

	<!-- Mobile: the running head names the page; its contents sheet lists
	     the others and signing out. Pages, not a scroll, so no hairline. -->
	<RunningHead
		entries={pages}
		current={active}
		expanded={contentsOpen}
		onopen={() => (contentsOpen = true)}
		class="top-0 mt-2"
	/>
	<ContentsSheet bind:open={contentsOpen} entries={pages} current={active} {actions} />

	<div class="mt-5 md:grid md:grid-cols-[200px_minmax(0,720px)] md:items-start md:gap-14">
		<nav
			aria-label={m.settings_sections_label()}
			class="sticky top-6 hidden flex-col gap-1.5 text-body-sm font-medium md:flex"
		>
			{#each pages as item (item.id)}
				<a
					href={item.href}
					aria-current={active === item.id ? 'page' : undefined}
					class="rounded-pill px-3.5 py-2 text-left transition {active === item.id
						? 'bg-surface font-semibold text-text'
						: 'text-text-muted hover:text-text'}"
				>
					{item.label}
				</a>
			{/each}
			<div class="mx-3.5 my-1 h-px bg-border" aria-hidden="true"></div>
			<button
				type="button"
				onclick={() => void signOut()}
				class="rounded-pill px-3.5 py-2 text-left text-destructive transition hover:bg-surface"
			>
				{m.logout()}
			</button>
		</nav>
		<div class="space-y-5">
			{@render children()}
		</div>
	</div>
</div>
