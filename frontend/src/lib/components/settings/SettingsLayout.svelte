<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import { session } from '$lib/auth.svelte';
	import { m } from '$lib/paraglide/messages';
	import { shell } from '$lib/shell.svelte';
	import { signOut } from '$lib/sign-out';

	let { active, children }: { active: 'profile' | 'users'; children: Snippet } = $props();

	const isAdmin = $derived(session.user?.role === 'admin');

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
</script>

{#snippet noActions()}{/snippet}

{#snippet navLinks(item: string, activeClass: string, idle: string, logout: string)}
	<a
		href={resolve('/settings')}
		aria-current={active === 'profile' ? 'page' : undefined}
		class="{item} {active === 'profile' ? activeClass : idle}"
	>
		{m.settings_nav_profile()}
	</a>
	{#if isAdmin}
		<a
			href={resolve('/settings/users')}
			aria-current={active === 'users' ? 'page' : undefined}
			class="{item} {active === 'users' ? activeClass : idle}"
		>
			{m.settings_nav_users()}
		</a>
	{/if}
	<button type="button" onclick={() => void signOut()} class="{item} {logout}">
		{m.logout()}
	</button>
{/snippet}

<div class="pt-6 pb-8 md:pt-10">
	<h1 class="font-display text-display-sm font-medium md:text-display-lg">{m.settings_title()}</h1>

	<!-- Mobile: horizontally scrollable chips, like the editor's section chips. -->
	<div class="mt-4 -mr-5 flex gap-1.5 overflow-x-auto pr-5 pb-1 md:hidden">
		{@render navLinks(
			'h-[34px] shrink-0 rounded-pill px-3.5 text-caption font-semibold transition',
			'bg-inverse text-inverse-foreground',
			'bg-surface text-text-muted',
			'bg-surface text-destructive'
		)}
	</div>

	<div class="mt-5 md:grid md:grid-cols-[200px_minmax(0,720px)] md:items-start md:gap-14">
		<nav
			aria-label={m.settings_sections_label()}
			class="sticky top-6 hidden flex-col gap-1.5 text-body-sm font-medium md:flex"
		>
			{@render navLinks(
				'rounded-pill px-3.5 py-2 text-left transition',
				'bg-surface font-semibold text-text',
				'text-text-muted hover:text-text',
				'text-destructive hover:bg-surface'
			)}
		</nav>
		<div class="space-y-5">
			{@render children()}
		</div>
	</div>
</div>
