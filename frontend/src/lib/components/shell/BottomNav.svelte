<script lang="ts">
	import BookOpen from 'lucide-svelte/icons/book-open';
	import Menu from 'lucide-svelte/icons/menu';
	import Plus from 'lucide-svelte/icons/plus';
	import Search from 'lucide-svelte/icons/search';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { m } from '$lib/paraglide/messages';
	import UserMenu from './UserMenu.svelte';

	let moreOpen = $state(false);

	const isHome = $derived(page.url.pathname === '/');
	const isSearchFocused = $derived(isHome && page.url.searchParams.get('focus') === 'search');
	const isNew = $derived(page.url.pathname === '/recipes/new');
</script>

<nav
	class="fixed right-4 bottom-4 left-4 z-30 flex h-16 items-center justify-around rounded-pill bg-inverse px-2 shadow-card md:hidden"
>
	<a
		href={resolve('/')}
		aria-current={isHome && !isSearchFocused ? 'page' : undefined}
		class="flex flex-1 flex-col items-center gap-1 text-label font-semibold transition {isHome &&
		!isSearchFocused
			? 'text-inverse-foreground'
			: 'text-inverse-muted'}"
	>
		<BookOpen class="size-5" aria-hidden="true" />
		{m.nav_recipes()}
	</a>
	<a
		href={resolve('/?focus=search')}
		aria-current={isSearchFocused ? 'page' : undefined}
		class="flex flex-1 flex-col items-center gap-1 text-label font-semibold transition {isSearchFocused
			? 'text-inverse-foreground'
			: 'text-inverse-muted'}"
	>
		<Search class="size-5" aria-hidden="true" />
		{m.nav_search()}
	</a>
	<!-- No icon/label column like the other items: the 48px circle already
	     reads as the "add" action on its own, and stacking a label under it
	     would either overflow the 64px bar or sit noticeably lower than the
	     other three labels. Centered on the bar instead, label moved to
	     `aria-label` so the control keeps an accessible name. -->
	<a
		href={resolve('/recipes/new')}
		aria-current={isNew ? 'page' : undefined}
		aria-label={m.nav_new()}
		class="flex flex-1 items-center justify-center"
	>
		<span
			aria-hidden="true"
			class="flex size-12 items-center justify-center rounded-full bg-primary text-primary-foreground"
		>
			<Plus class="size-5" />
		</span>
	</a>
	<button
		type="button"
		onclick={() => (moreOpen = true)}
		class="flex flex-1 flex-col items-center gap-1 text-label font-semibold transition {moreOpen
			? 'text-inverse-foreground'
			: 'text-inverse-muted'}"
	>
		<Menu class="size-5" aria-hidden="true" />
		{m.nav_more()}
	</button>
</nav>

<UserMenu variant="sheet" bind:open={moreOpen} />
