<script lang="ts">
	import type { UserAccount, UserRole } from '$lib/api/users';
	import Button from '$lib/components/ui/Button.svelte';
	import Select from '$lib/components/ui/Select.svelte';
	import { m } from '$lib/paraglide/messages';

	let {
		user,
		isSelf,
		isLastAdmin,
		onrole,
		onreset,
		ondelete
	}: {
		user: UserAccount;
		isSelf: boolean;
		isLastAdmin: boolean;
		/** Rejects when the API refused; the select then snaps back. */
		onrole: (user: UserAccount, role: UserRole) => Promise<void>;
		onreset: (user: UserAccount) => void;
		ondelete: (user: UserAccount) => void;
	} = $props();

	// Writable derived (Svelte >= 5.25): follows `user.role` from the parent's
	// list, but can be overridden locally so a refused change snaps back even
	// though the parent's value never changed.
	let role = $derived<string>(user.role);

	const roleOptions = [
		{ value: 'admin', label: m.users_role_admin() },
		{ value: 'user', label: m.users_role_member() }
	];
	const initial = $derived(user.username.charAt(0).toUpperCase());
	const pillClass = $derived(
		role === 'admin' ? 'bg-accent text-accent-foreground' : 'bg-background text-text-muted'
	);

	async function changeRole(next: string) {
		try {
			await onrole(user, next as UserRole);
		} catch {
			role = user.role;
		}
	}
</script>

<!--
	The name takes the top line; role and actions share the next one and wrap
	only when they have to - a row with just "Löschen" (182px) stays on one
	line, one that also offers "Passwort zurücksetzen" (346px) breaks, since a
	360px card holds 302px. `md:contents` dissolves that wrapper on desktop so
	role and actions become the grid's own second and third column. The
	separator belongs between entries, so the last row drops it.
-->
<li
	class="grid grid-cols-[1fr_auto] items-center gap-3 border-b border-dashed border-border py-4 last:border-b-0 md:grid-cols-[1fr_140px_280px]"
>
	<div class="col-span-2 flex items-center gap-3 md:col-span-1">
		<span
			aria-hidden="true"
			class="flex size-8 shrink-0 items-center justify-center rounded-full bg-accent font-display font-semibold text-accent-foreground"
		>
			{initial}
		</span>
		<span class="text-body font-medium">{user.username}</span>
		{#if isSelf}
			<span class="text-micro text-text-muted">{m.users_you()}</span>
		{/if}
	</div>

	<div class="col-span-2 flex flex-wrap items-center justify-between gap-2 md:contents">
		<Select
			bind:value={role}
			options={roleOptions}
			label={m.users_role_aria({ username: user.username })}
			disabled={isSelf || isLastAdmin}
			onchange={changeRole}
			class="{pillClass} h-11 md:h-8"
		/>

		<!-- 44px tall while a thumb is doing the tapping, back to the table's own
	     density on desktop. "Löschen" carries its own padded pill for the same
	     reason: bare text is a 50×20 target. -->
		<div class="flex items-center gap-2 md:justify-end">
			<!-- Not for the own account: PATCH /users/:id ends every session of
		     the target, so resetting the own password would sign this admin
		     out. The profile page's password form is the way to do that. -->
			{#if !isSelf}
				<Button
					variant="secondary"
					class="h-11 px-3 text-caption whitespace-nowrap md:h-8"
					label={m.users_reset_password_aria({ username: user.username })}
					onclick={() => onreset(user)}
				>
					{m.users_reset_password()}
				</Button>
			{/if}
			<button
				type="button"
				aria-label={m.users_delete_aria({ username: user.username })}
				disabled={isSelf || isLastAdmin}
				onclick={() => ondelete(user)}
				class="inline-flex h-11 items-center rounded-pill px-3 text-caption font-semibold whitespace-nowrap text-destructive transition hover:bg-destructive-soft focus-visible:outline-2 focus-visible:outline-offset-2 focus-visible:outline-primary disabled:cursor-not-allowed disabled:text-handle disabled:hover:bg-transparent md:h-8"
			>
				{m.common_delete()}
			</button>
		</div>
	</div>
	{#if isLastAdmin}
		<p class="col-span-2 text-micro text-text-muted md:col-span-1 md:col-start-3 md:text-right">
			{m.users_last_admin_hint()}
		</p>
	{/if}
</li>
