<script lang="ts">
	import { Collapsible } from 'bits-ui';
	import Check from '@lucide/svelte/icons/check';
	import ChevronDown from '@lucide/svelte/icons/chevron-down';
	import LinkIcon from '@lucide/svelte/icons/link';
	import { onMount } from 'svelte';
	import { listUsers, type UserAccount } from '$lib/api/users';
	import PersonMark from '$lib/components/ui/PersonMark.svelte';
	import { m } from '$lib/paraglide/messages';

	// Who a switch or turning off leaves behind, by face and name. The
	// warning text says how many; this says who. Loaded on open, so it is
	// the list as it is now rather than when the page loaded.
	let accounts = $state<UserAccount[]>([]);

	onMount(async () => {
		try {
			accounts = (await listUsers()).filter((user) => user.hasIdentity);
		} catch {
			// The warning text already carries the counts; the box is extra.
		}
	});

	// Three faces and a count: more would push the summary off a phone.
	const faces = $derived(accounts.slice(0, 3));
	const more = $derived(accounts.length - faces.length);
	const without = $derived(accounts.filter((user) => !user.hasPassword).length);
</script>

{#snippet group(
	people: UserAccount[],
	title: (inputs: { count: number }) => string,
	hint: string,
	keeps: boolean
)}
	{#if people.length > 0}
		<div class="border-t border-border pb-1.5">
			<p
				class="flex items-start gap-2 px-3 pt-2.5 pb-1 text-caption {keeps
					? 'text-success'
					: 'text-destructive'}"
			>
				{#if keeps}
					<Check class="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
				{:else}
					<LinkIcon class="mt-0.5 size-3.5 shrink-0" aria-hidden="true" />
				{/if}
				<span>
					<span class="font-semibold">{title({ count: people.length })}</span>
					<span class="text-text-muted">{hint}</span>
				</span>
			</p>
			<ul>
				{#each people as person (person.id)}
					<li class="flex items-center gap-3 px-3 py-1.5">
						<PersonMark {person} size="sm" />
						<span class="min-w-0 flex-1">
							<span class="block text-body-sm font-semibold wrap-anywhere"
								>{person.displayName}</span
							>
							<span class="block text-caption wrap-anywhere text-text-muted"
								>@{person.username}</span
							>
						</span>
					</li>
				{/each}
			</ul>
		</div>
	{/if}
{/snippet}

{#if accounts.length > 0}
	<Collapsible.Root
		class="@container mt-4 overflow-hidden rounded-lg border border-border bg-surface-elevated"
	>
		<Collapsible.Trigger
			class="group grid w-full grid-cols-[1fr_auto] items-center gap-x-3 gap-y-2 px-3 py-2.5 text-left focus-visible:outline-2 focus-visible:-outline-offset-2 focus-visible:outline-primary @xs:grid-cols-[auto_1fr_auto]"
		>
			<!-- Narrow, as on a phone, the faces share the top line with the chevron
			     and the text gets the full width below; from 20rem they sit in one row. -->
			<span class="flex">
				{#each faces as person, index (person.id)}
					<PersonMark
						{person}
						size="sm"
						class="ring-2 ring-surface-elevated {index > 0 ? '-ml-2' : ''}"
					/>
				{/each}
				{#if more > 0}
					<!-- `relative` like PersonMark, so the count lies on top of the faces. -->
					<span
						aria-hidden="true"
						class="relative -ml-2 flex size-7 items-center justify-center rounded-full border border-border bg-surface text-micro font-semibold text-text-muted ring-2 ring-surface-elevated"
						>+{more}</span
					>
				{/if}
			</span>
			<span
				class="col-span-2 row-start-2 min-w-0 text-body-sm font-semibold @xs:col-span-1 @xs:col-start-2 @xs:row-start-1"
			>
				{accounts.length === 1
					? m.settings_sign_in_accounts_one()
					: m.settings_sign_in_accounts({ count: accounts.length })}
				{#if without > 0}
					<span class="block text-caption font-normal text-destructive">
						{without === 1
							? m.settings_sign_in_accounts_need_link_one()
							: m.settings_sign_in_accounts_need_link({ count: without })}
					</span>
				{/if}
			</span>
			<ChevronDown
				class="col-start-2 row-start-1 size-4 text-text-muted transition-transform group-data-[state=open]:rotate-180 motion-reduce:transition-none @xs:col-start-3"
				aria-hidden="true"
			/>
		</Collapsible.Trigger>
		<!-- Two groups, who needs a setup link first: what turning off costs each
		     person is said once per group, not on every row. Every account,
		     however many: the dialog scrolls as a whole, and a scroll area of its
		     own hid its scrollbar and looked cut off. -->
		<Collapsible.Content>
			{@render group(
				accounts.filter((user) => !user.hasPassword),
				m.settings_sign_in_accounts_group_without,
				m.settings_sign_in_accounts_hint_without(),
				false
			)}
			{@render group(
				accounts.filter((user) => user.hasPassword),
				m.settings_sign_in_accounts_group_with,
				m.settings_sign_in_accounts_hint_with(),
				true
			)}
		</Collapsible.Content>
	</Collapsible.Root>
{/if}
