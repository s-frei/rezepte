<script lang="ts">
	import { session } from '#lib/auth.svelte.js';
	import type { ShoppingList } from '#lib/api/shopping.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let { list }: { list: Pick<ShoppingList, 'owner' | 'members'> } = $props();

	const me = $derived(session.user?.id);
	const people = $derived([
		{
			person: list.owner,
			role: list.owner.id === me ? m.list_side_you_created() : m.list_side_created()
		},
		...list.members.map((person) => ({
			person,
			role: person.id === me ? m.list_side_you_member() : m.list_side_member()
		}))
	]);
</script>

<!-- Who is on a list: the creator first, then the members; in the side card and the people sheet. -->
<ul>
	{#each people as { person, role } (person.id)}
		<li class="flex items-center gap-3 py-2">
			<PersonMark {person} size="sm" />
			<div class="min-w-0">
				<p class="truncate text-body font-semibold">{person.displayName}</p>
				<p class="text-caption text-text-muted">{role}</p>
			</div>
		</li>
	{/each}
</ul>
