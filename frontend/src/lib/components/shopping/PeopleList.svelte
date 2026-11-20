<script lang="ts">
	import { session } from '#lib/auth.svelte.js';
	import type { ListPerson } from '#lib/api/shopping.js';
	import PersonMark from '#lib/components/ui/PersonMark.svelte';
	import { m } from '#lib/paraglide/messages.js';

	let {
		owner,
		members,
		kind = 'list'
	}: {
		owner: ListPerson;
		members: ListPerson[];
		/** What they share, which words the roles. */
		kind?: 'list' | 'plan';
	} = $props();

	const me = $derived(session.user?.id);
	const roles = $derived(
		kind === 'plan'
			? {
					created: m.plan_side_created(),
					youCreated: m.plan_side_you_created(),
					member: m.plan_side_member(),
					youMember: m.plan_side_you_member()
				}
			: {
					created: m.list_side_created(),
					youCreated: m.list_side_you_created(),
					member: m.list_side_member(),
					youMember: m.list_side_you_member()
				}
	);
	const people = $derived([
		{ person: owner, role: owner.id === me ? roles.youCreated : roles.created },
		...members.map((person) => ({
			person,
			role: person.id === me ? roles.youMember : roles.member
		}))
	]);
</script>

<!-- Who is on a list or a plan: the creator first, then the members; in the side card and the people sheet. -->
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
