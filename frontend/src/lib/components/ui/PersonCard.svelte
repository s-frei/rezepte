<script lang="ts">
	import { Dialog } from 'bits-ui';
	import { resolve } from '$app/paths';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';
	import { m } from '$lib/paraglide/messages';
	import { roleLabel } from '$lib/roles';
	import { personDetails } from '$lib/user/people.svelte';

	let {
		person,
		compact = false,
		asTitle = false,
		onnavigate
	}: {
		person: {
			id: string;
			username: string;
			displayName: string;
			color: string;
			avatarId: string | null;
		};
		/** One line per person, for the author panel that stacks two. */
		compact?: boolean;
		/** The name names the dialog the card sits in (the people sheet). */
		asTitle?: boolean;
		/** Called when the "Recipes by" link is followed, so a popover can
		 * close: the card may stay in the filtered list it leads to. */
		onnavigate?: () => void;
	} = $props();

	const details = personDetails();
	$effect(() => {
		void details.load();
	});

	const role = $derived(details.roles.get(person.id));
	const count = $derived(details.counts.get(person.username) ?? 0);
	const nameClass = 'truncate font-display text-body font-medium';
</script>

<div class="flex items-center gap-3">
	<PersonMark {person} size={compact ? 'lg' : 'card'} />
	<div class="min-w-0">
		{#if asTitle}
			<Dialog.Title class={nameClass}>{person.displayName}</Dialog.Title>
		{:else}
			<p class={nameClass}>{person.displayName}</p>
		{/if}
		<p class="truncate text-caption opacity-80">
			@{person.username}{#if role}&ensp;·&ensp;{roleLabel(role)}{/if}
		</p>
		<!-- No link for someone without recipes: it would lead to an empty filter. -->
		{#if details.loaded && count === 0}
			<p class="text-caption">{m.person_card_recipes({ count })}</p>
		{:else if details.loaded}
			<a
				href="{resolve('/')}?author={encodeURIComponent(person.username)}"
				class="text-caption underline underline-offset-2"
				onclick={onnavigate}
			>
				{count === 1 ? m.person_card_recipes_one() : m.person_card_recipes({ count })}
				&ensp;→&ensp;{m.person_card_recipes_by({ name: person.displayName })}
			</a>
		{/if}
	</div>
</div>
