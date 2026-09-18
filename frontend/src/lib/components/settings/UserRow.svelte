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

<li
	class="grid grid-cols-1 gap-3 border-b border-dashed border-border py-4 md:grid-cols-[1fr_140px_240px] md:items-center"
>
	<div class="flex items-center gap-3">
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

	<div>
		<Select
			bind:value={role}
			options={roleOptions}
			label={m.users_role_aria({ username: user.username })}
			disabled={isSelf || isLastAdmin}
			onchange={changeRole}
			class={pillClass}
		/>
	</div>

	<div class="flex flex-col items-start gap-1 md:items-end">
		<div class="flex items-center gap-3">
			<!-- Not for the own account: PATCH /users/:id ends every session of
			     the target, so resetting the own password would sign this admin
			     out. The profile page's password form is the way to do that. -->
			{#if !isSelf}
				<Button
					variant="secondary"
					class="h-8 px-3 text-caption"
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
				class="text-caption font-semibold text-destructive transition hover:brightness-95 disabled:cursor-not-allowed disabled:text-handle"
			>
				{m.common_delete()}
			</button>
		</div>
		{#if isLastAdmin}
			<p class="text-micro text-text-muted">{m.users_last_admin_hint()}</p>
		{/if}
	</div>
</li>
