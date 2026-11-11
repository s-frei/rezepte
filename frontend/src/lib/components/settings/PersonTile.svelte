<script lang="ts">
	import { cubicOut } from 'svelte/easing';
	import { slide } from 'svelte/transition';
	import BookOpen from '@lucide/svelte/icons/book-open';
	import ChevronRight from '@lucide/svelte/icons/chevron-right';
	import Crown from '@lucide/svelte/icons/crown';
	import KeyRound from '@lucide/svelte/icons/key-round';
	import Link from '@lucide/svelte/icons/link';
	import MailCheck from '@lucide/svelte/icons/mail-check';
	import MailQuestionMark from '@lucide/svelte/icons/mail-question-mark';
	import ShoppingBasket from '@lucide/svelte/icons/shopping-basket';
	import { resolve } from '$app/paths';
	import { imageUrl } from '#lib/api/recipes.js';
	import type { PersonEntry, UserRole } from '#lib/api/users.js';
	import PersonCard from '#lib/components/ui/PersonCard.svelte';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import PersonPeek from '#lib/components/ui/PersonPeek.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { formatDate } from '#lib/recipe/format.js';
	import { initialFor, tintFor } from '#lib/recipe/placeholder.js';
	import { isAdminRole, roleLabel } from '#lib/roles.js';
	import { lastActive } from '#lib/settings/last-active.js';
	import { narrow } from '#lib/ui/narrow.js';
	import { canOpenPerson, listsText } from '#lib/settings/person-page.js';
	import { rowPermissions } from '#lib/settings/row-permissions.js';

	let {
		person,
		isSelf,
		actorRole,
		hasPassword,
		hasIdentity,
		provider,
		setupLinkExpiresAt,
		email,
		emailVerified,
		lastActiveAt,
		/** How long the card opens or closes; 0 unless the viewer just added it (`rowMotion`). */
		motion = 0
	}: {
		person: PersonEntry;
		isSelf: boolean;
		/** The signed-in user's role: it decides where the card leads. */
		actorRole: UserRole;
		/** Admin-only, like every field down to `lastActiveAt`: undefined
		 * where the viewer cannot see it (a member's view of the list). */
		hasPassword?: boolean;
		hasIdentity?: boolean;
		/** The identity provider's name; undefined while this instance has none. */
		provider?: string;
		/** Set while this person has an open setup link. */
		setupLinkExpiresAt?: string;
		email?: string;
		emailVerified?: boolean;
		lastActiveAt?: string | null;
		motion?: number;
	} = $props();

	let cardOpen = $state(false);

	const permissions = $derived(rowPermissions(actorRole, person, isSelf));
	// The own card leads to the profile, anyone the viewer may act on to
	// their page; any other card leads nowhere and its circle opens who the
	// person is.
	const href = $derived(
		isSelf
			? resolve('/settings')
			: canOpenPerson(permissions)
				? resolve('/settings/users/[username]', { username: person.username })
				: undefined
	);
	// Admin-only like the address: `lastActiveAt` comes from `GET /users`.
	const activity = $derived(isAdminRole(actorRole) ? lastActive(lastActiveAt) : null);
	const setupStatus = $derived(
		!permissions.manageAccount
			? null
			: setupLinkExpiresAt
				? m.users_setup_link_open({ date: formatDate(setupLinkExpiresAt) })
				: !hasPassword && !hasIdentity
					? m.users_not_set_up()
					: null
	);
	// Its own line: an account can carry both at once (an open link on top of
	// an already-connected identity), and it is a standing fact, not a
	// transient one.
	const identityStatus = $derived(
		permissions.manageAccount && hasIdentity && provider
			? m.users_identity_status({ name: provider })
			: null
	);

	const lists = $derived(person.listCount ?? 0);
	const recipes = $derived(person.recipeCount ?? 0);
	const latest = $derived(person.latestRecipes ?? []);

	const emailLabel = $derived(
		emailVerified ? m.users_email_confirmed() : m.users_email_unconfirmed()
	);

	// The role pill: one word, never broken; below a narrow card it takes
	// its own line, which holds it even at 200% text on a phone.
	const pill =
		'ml-auto inline-flex min-h-8 max-w-full items-center gap-1.5 rounded-pill px-3 text-body-sm font-semibold whitespace-nowrap';

	// The number in bold, the words around it as the catalog has them.
	function split(text: string, count: number): [string, string, string] {
		const at = count === 0 ? -1 : text.indexOf(String(count));
		if (at < 0) return [text, '', ''];
		return [text.slice(0, at), String(count), text.slice(at + String(count).length)];
	}
</script>

<!--
	One card per person: the circle in their own color, the display name and
	the login name with the address's state - what tells two people called
	"Mia" apart - and for an admin how long ago they were active. Under a
	dashed rule the covers of their newest recipes with the newest one's
	title, then their shopping lists and the role. A card the viewer may open
	is one link (the own card to the profile); on any other the circle opens
	who the person is.
-->
<li class="@container" transition:slide={{ duration: motion, easing: cubicOut }}>
	{#if href}
		<a
			{href}
			data-person={person.username}
			class="block rounded-xl border border-border hover-tint p-3.5 transition focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary @lg:px-4 @lg:pb-4"
		>
			{@render content(false)}
		</a>
	{:else}
		<div
			data-person={person.username}
			class="rounded-xl border border-border p-3.5 @lg:px-4 @lg:pb-4"
		>
			{@render content(true)}
		</div>
	{/if}
</li>

<!-- `interactive`: the circle opens who the person is; false inside the
     card's own link, which ends in a chevron. -->
{#snippet content(interactive: boolean)}
	<!-- A grid, so the status renders once: below a 512px card it sits under the login
	     name, from 512px between the names and the chevron. Below a 13rem card
	     (200% text on a phone) the circle sits above the names, which take the
	     card's full width instead of breaking a letter per line. Inside the
	     card's link the circle is decoration: the name already says who. -->
	<div
		class="grid grid-cols-[minmax(0,1fr)_auto] items-center gap-x-2.5 gap-y-2 @[13rem]:grid-cols-[auto_minmax(0,1fr)_auto] @[13rem]:gap-y-0 @lg:grid-cols-[auto_minmax(0,1fr)_auto_auto] @lg:gap-x-3"
	>
		<span
			aria-hidden={interactive ? undefined : 'true'}
			class="col-start-1 row-start-1 flex @[13rem]:row-span-2 @lg:row-span-1"
		>
			{#if !interactive}
				<PersonMark {person} size="lg" class="@lg:size-11" />
			{:else}
				<!-- The phone's circle hits on 4px more around it. -->
				<PersonPeek
					bind:open={cardOpen}
					hover={false}
					label={m.person_card_open({ name: person.displayName })}
					class="{narrow.current
						? 'relative after:absolute after:-inset-1'
						: ''} shrink-0 rounded-full focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary"
					panelClass="max-w-80 bg-surface p-4 text-text"
				>
					{#snippet trigger()}
						<PersonMark {person} size="lg" class="@lg:size-11" />
					{/snippet}
					{#snippet card(sheet)}
						{#if sheet}
							<PersonCard {person} asTitle onnavigate={() => (cardOpen = false)} />
						{:else}
							<PersonCard {person} />
						{/if}
					{/snippet}
				</PersonPeek>
			{/if}
		</span>
		<span
			class="col-span-2 row-start-2 block min-w-0 @[13rem]:col-span-1 @[13rem]:col-start-2 @[13rem]:row-start-1"
			>{@render who()}</span
		>
		{#if href}
			<ChevronRight
				class="col-start-2 row-start-1 size-[18px] text-text-muted @[13rem]:col-start-3 @[13rem]:row-span-2 @lg:col-start-4 @lg:row-span-1"
				aria-hidden="true"
			/>
		{/if}
		{#if activity}
			<span
				class="col-span-2 row-start-3 mt-1 flex min-w-0 @[13rem]:col-span-1 @[13rem]:col-start-2 @[13rem]:row-start-2 @lg:col-start-3 @lg:row-start-1 @lg:mt-0"
			>
				{@render status()}
			</span>
		{/if}
	</div>

	{#if setupStatus}
		<p
			class="mt-2.5 flex items-start gap-1.5 rounded-md border border-dashed border-border px-3 py-2 text-micro text-text-muted"
		>
			<Link class="mt-0.5 size-3 shrink-0" aria-hidden="true" />
			<span class="min-w-0 wrap-break-word hyphens-auto">{setupStatus}</span>
		</p>
	{/if}

	<!-- Recipes before shopping lists, everywhere the two stand together.
	     Below a 512px card the lists and the role share a line under the recipes,
	     the role at its end; below 17rem of card (a 320px phone, 200% text) the
	     role always takes a line of its own, so no card has it beside the
	     lists while the next has it under them. From 512px they keep the
	     line's end. -->
	<div
		class="mt-3.5 flex flex-col gap-3 border-t border-dashed border-border pt-3.5 @lg:flex-row @lg:flex-wrap @lg:items-center @lg:gap-x-3"
	>
		{#if latest.length > 0}
			<div
				data-latest
				class="flex min-w-0 flex-col items-start gap-2.5 @lg:flex-row @lg:items-center @lg:gap-3.5"
			>
				<!-- Newest first and larger; plain pictures, the title says which.
				     4px apart in a card below 240px, so four fit at 320px; below
				     15rem of card (200% text) only the newest, which still fits. -->
				<span class="flex shrink-0 items-end gap-1 @[15rem]:gap-1.5">
					{#each latest as recipe, i (recipe.id)}
						{@const size =
							i === 0 ? 'size-14 @lg:size-15' : 'size-10 @lg:size-11 @max-[15rem]:hidden'}
						{#if recipe.coverImageId}
							<img
								data-thumb
								src={imageUrl(recipe.id, recipe.coverImageId, 'thumb')}
								alt=""
								loading="lazy"
								decoding="async"
								class="block shrink-0 rounded-md bg-skeleton object-cover {size}"
							/>
						{:else}
							<span
								data-thumb
								aria-hidden="true"
								class="flex shrink-0 items-center justify-center rounded-md font-display font-medium text-primary italic {tintFor(
									recipe.id
								)} {size} {i === 0 ? 'text-heading' : 'text-body-lg'}"
								>{initialFor(recipe.title)}</span
							>
						{/if}
					{/each}
				</span>
				<span class="block max-w-full min-w-0">
					<span class="block font-display text-card-sm font-medium wrap-break-word hyphens-auto">
						{latest[0].title}
					</span>
					<span class="mt-0.5 block text-caption text-text-muted @lg:whitespace-nowrap">
						{recipes > 1 ? m.users_latest_of({ count: recipes }) : m.users_latest_only()}
					</span>
				</span>
			</div>
		{:else}
			<p>{@render fact(BookOpen, 0, m.users_recipes_none_yet())}</p>
		{/if}
		<div
			class="flex flex-wrap items-center justify-between gap-x-3.5 gap-y-2 @max-[17rem]:flex-col @max-[17rem]:items-start @lg:ml-auto"
		>
			{@render fact(ShoppingBasket, lists, listsText(lists))}
			{#if person.role === 'superadmin'}
				<span class="{pill} bg-accent text-accent-foreground">
					<Crown class="size-3.5 shrink-0" aria-hidden="true" />
					<span class="min-w-0">{m.users_role_owner()}</span>
				</span>
			{:else}
				<span
					class="{pill} {person.role === 'admin'
						? 'bg-accent text-accent-foreground'
						: 'bg-background text-text-muted'}"
				>
					<span class="min-w-0">{roleLabel(person.role)}</span>
				</span>
			{/if}
		</div>
	</div>
{/snippet}

{#snippet who()}
	<!-- "you" flows after the name's last word rather than in a column
	     of its own, which squeezed the name on a 320px phone. -->
	<span class="block text-body font-medium wrap-break-word hyphens-auto">
		{person.displayName}
		{#if isSelf}
			<span class="ml-1 text-micro font-normal whitespace-nowrap text-text-muted"
				>{m.users_you()}</span
			>
		{/if}
	</span>
	<!-- The address's state in one symbol after the login name: what a
	     forgotten-password mail can reach. None without an address. -->
	<span class="flex min-w-0 items-center gap-1 text-micro text-text-muted">
		<span class="truncate">{person.username}</span>
		{#if email}
			<span class="inline-flex shrink-0" title={emailLabel}>
				{#if emailVerified}
					<MailCheck class="size-3 shrink-0 text-success-foreground" aria-hidden="true" />
				{:else}
					<MailQuestionMark class="size-3 shrink-0 text-text-muted" aria-hidden="true" />
				{/if}
				<span class="sr-only">{emailLabel}</span>
			</span>
		{/if}
	</span>
	{#if identityStatus}
		<span class="flex items-center gap-1.5 text-micro text-text-muted">
			<KeyRound
				class="mt-0.5 size-3 shrink-0 self-start @lg:mt-0 @lg:self-center"
				aria-hidden="true"
			/>
			<span class="min-w-0">{identityStatus}</span>
		</span>
	{/if}
{/snippet}

<!-- The dot says the state at a glance: green within a week, the attention
     ink once dormant (30 days or more), a hollow ring when nothing is
     recorded, muted in between. It stays on the text's first line when the
     text wraps. -->
{#snippet status()}
	{#if activity}
		<span
			data-activity={activity.state}
			class="flex min-w-0 items-start gap-1.5 text-micro {activity.dormant
				? 'font-semibold text-accent-foreground'
				: 'text-text-muted'}"
		>
			<!-- As wide as the sign-in line's 12px key, so both texts start together. -->
			<span class="flex h-[1lh] w-3 shrink-0 items-center">
				<span
					class="size-[7px] rounded-full {activity.state === 'recent'
						? 'bg-success-foreground'
						: activity.state === 'dormant'
							? 'bg-accent-foreground'
							: activity.state === 'none'
								? 'inset-ring-[1.5px] inset-ring-text-muted'
								: 'bg-text-muted/60'}"
				></span>
			</span>
			<span class="min-w-0">{activity.text}</span>
		</span>
	{/if}
{/snippet}

{#snippet fact(Icon: typeof BookOpen, count: number, text: string)}
	{@const [before, number, after] = split(text, count)}
	<!-- The icon stays on the first line when the words wrap (200% text). -->
	<span class="inline-flex max-w-full min-w-0 items-start gap-1.5 text-caption text-text-muted">
		<span class="flex h-[1lh] shrink-0 items-center">
			<Icon class="size-3.5" aria-hidden="true" />
		</span>
		<span class="min-w-0 wrap-break-word hyphens-auto"
			>{before}<b class="font-semibold text-text">{number}</b>{after}</span
		>
	</span>
{/snippet}
