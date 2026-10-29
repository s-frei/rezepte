<script lang="ts" module>
	// Where the current pill sat on the settings page just left, and when:
	// every settings page mounts its own navigation, so the new pill reads
	// the old one to glide from its row. Only a page change a moment ago
	// counts; coming back to settings later starts the pill in place.
	let leftPill: { top: number; at: number } | null = null;
</script>

<script lang="ts">
	import type { Snippet } from 'svelte';
	import LogOut from '@lucide/svelte/icons/log-out';
	import ContentsSheet from '$lib/components/nav/ContentsSheet.svelte';
	import { byGroup } from '$lib/components/nav/contents';
	import GroupHeading from '$lib/components/nav/GroupHeading.svelte';
	import RunningHead from '$lib/components/nav/RunningHead.svelte';
	import { session } from '$lib/auth.svelte';
	import { m } from '$lib/paraglide/messages';
	import { settingsPages, type SettingsPageId } from '$lib/settings/pages';
	import { shell } from '$lib/shell.svelte';
	import { signOut } from '$lib/sign-out';
	import { prefersReducedMotion } from 'svelte/motion';
	import { beforeNavigate } from '$app/navigation';

	let { active, children }: { active: SettingsPageId; children: Snippet } = $props();

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

	const pages = $derived(settingsPages(session.user?.role));

	let contentsOpen = $state(false);
	// Signing out is not a page, so on a phone it is an action at the foot of
	// the contents sheet, set apart the way the sidebar sets it apart.
	const actions = [
		{
			id: 'sign-out',
			label: m.logout(),
			tone: 'destructive' as const,
			icon: LogOut,
			onselect: () => void signOut()
		}
	];

	// Glides the pill from the row it sat on to this one; the labels stay
	// where they are. The old row is measured when the navigation starts,
	// while the old page is still laid out: read as the old page was torn
	// down, it could already be out of the document, measured 0, and the
	// pill came in from the top of the list. Rows are measured from the top
	// of their navigation, since the new page mounts while the old one is
	// still in the document above it.
	let currentPill: HTMLElement | undefined;

	function offset(node: HTMLElement) {
		const nav = node.closest('nav');
		return node.getBoundingClientRect().top - (nav?.getBoundingClientRect().top ?? 0);
	}

	beforeNavigate(() => {
		if (currentPill?.isConnected && currentPill.getClientRects().length > 0) {
			leftPill = { top: offset(currentPill), at: performance.now() };
		}
	});

	function pill(node: HTMLElement) {
		if (leftPill && performance.now() - leftPill.at < 2000 && !prefersReducedMotion.current) {
			const easing = getComputedStyle(node).getPropertyValue('--ease-glide').trim() || 'ease';
			node.animate(
				[{ transform: `translateY(${leftPill.top - offset(node)}px)` }, { transform: 'none' }],
				{ duration: 320, easing }
			);
		}
		leftPill = null;
		currentPill = node;
		return () => {
			if (currentPill === node) currentPill = undefined;
		};
	}
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
			class="sticky top-6 isolate hidden flex-col gap-1.5 text-body-sm font-medium md:flex"
		>
			{#each byGroup(pages) as run, i (run.label)}
				<div role="group" aria-label={run.label} class="flex flex-col gap-1.5">
					<GroupHeading label={run.label} first={i === 0} class="px-3.5" />
					{#each run.entries as item (item.id)}
						{@const Icon = item.icon}
						{@const current = active === item.id}
						<!-- The bottom nav's current pill: one way to say "you are here"
						     across the app. The other icons stay muted, as in the
						     avatar menu. The pill is a layer of its own under every
						     label (the nav is the stacking context), and glides from
						     the old row to the new one (`pill`) while the labels stay
						     put. -->
						<a
							href={item.href}
							aria-current={current ? 'page' : undefined}
							class="relative flex items-center gap-2.5 rounded-pill px-3.5 py-2 text-left transition {current
								? 'font-semibold text-accent-foreground'
								: 'text-text-muted hover:text-text'}"
						>
							{#if current}
								<span
									aria-hidden="true"
									class="absolute inset-0 -z-10 rounded-pill bg-accent"
									{@attach pill}
								></span>
							{/if}
							<Icon class="size-4 shrink-0 {current ? '' : 'text-text-muted'}" aria-hidden="true" />
							{item.label}
						</a>
					{/each}
				</div>
			{/each}
			<div class="mx-3.5 my-1 h-px bg-border" aria-hidden="true"></div>
			<button
				type="button"
				onclick={() => void signOut()}
				class="flex items-center gap-2.5 rounded-pill px-3.5 py-2 text-left text-destructive transition hover:bg-surface"
			>
				<LogOut class="size-4 shrink-0" aria-hidden="true" />
				{m.logout()}
			</button>
		</nav>
		<div class="space-y-5">
			{@render children()}
		</div>
	</div>
</div>
