<script lang="ts">
	import { Dialog, RadioGroup } from 'bits-ui';
	import { toast } from 'svelte-sonner';
	import { fade, scale } from 'svelte/transition';
	import { ApiError, isSignedOut } from '$lib/api/client';
	import { createUser, type UserAccount, type UserRole } from '$lib/api/users';
	import Button from '$lib/components/ui/Button.svelte';
	import Input from '$lib/components/ui/Input.svelte';
	import { m } from '$lib/paraglide/messages';
	import { passwordErrorsFromApi, validateNewPassword } from '$lib/settings/password';

	let {
		open = $bindable(false),
		oncreated
	}: { open?: boolean; oncreated: (user: UserAccount) => void } = $props();

	let username = $state('');
	let password = $state('');
	let role = $state<string>('user');
	let errors = $state<{ username?: string; password?: string }>({});
	let saving = $state(false);

	const roles: { value: UserRole; label: string; hint: string }[] = [
		{ value: 'user', label: m.users_role_member(), hint: m.users_role_member_hint() },
		{ value: 'admin', label: m.users_role_admin(), hint: m.users_role_admin_hint() }
	];

	// A fresh form every time the dialog opens.
	$effect(() => {
		if (open) {
			username = '';
			password = '';
			role = 'user';
			errors = {};
		}
	});

	async function submit(event: SubmitEvent) {
		event.preventDefault();
		errors = {};
		if (username.trim() === '') {
			errors.username = m.users_validation_username();
		}
		const pw = validateNewPassword(password, password);
		if (pw.next) {
			errors.password = pw.next;
		}
		if (Object.keys(errors).length > 0) {
			return;
		}
		saving = true;
		try {
			const created = await createUser({
				username: username.trim(),
				password,
				role: role as UserRole
			});
			toast.success(m.users_created({ username: created.username }));
			open = false;
			oncreated(created);
		} catch (error) {
			if (error instanceof ApiError && error.status === 409) {
				toast.error(m.users_username_taken());
			} else if (error instanceof ApiError && error.status === 422) {
				if (error.errors.some((e) => e.location === 'body.username')) {
					errors.username = m.users_validation_username();
				}
				const password = passwordErrorsFromApi(error.errors).next;
				if (password) {
					errors.password = password;
				}
				if (Object.keys(errors).length === 0) toast.error(error.detail ?? m.users_create_error());
			} else if (!isSignedOut(error)) {
				// 401 already redirects to the login page (see $lib/api/client).
				toast.error(m.users_create_error());
			}
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Portal>
		<Dialog.Overlay forceMount>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed inset-0 z-40 bg-overlay"
						transition:fade={{ duration: 200 }}
					></div>
				{/if}
			{/snippet}
		</Dialog.Overlay>
		<Dialog.Content forceMount preventScroll={false}>
			{#snippet child({ props, open: isOpen })}
				{#if isOpen}
					<div
						{...props}
						class="fixed top-1/2 left-1/2 z-50 w-[calc(100%-2.5rem)] max-w-[440px] -translate-x-1/2 -translate-y-1/2 rounded-3xl bg-surface p-7 shadow-dialog"
						transition:scale={{ duration: 200, start: 0.95 }}
					>
						<Dialog.Title class="font-display text-heading font-medium">
							{m.users_create_title()}
						</Dialog.Title>
						<Dialog.Description class="sr-only">{m.users_create_description()}</Dialog.Description>
						<form onsubmit={submit} class="mt-5 space-y-4">
							<Input
								id="new-user-name"
								label={m.login_username()}
								autocomplete="off"
								bind:value={username}
								error={errors.username ?? null}
							/>
							<Input
								id="new-user-password"
								label={m.login_password()}
								type="password"
								autocomplete="new-password"
								bind:value={password}
								error={errors.password ?? null}
							/>
							<RadioGroup.Root
								bind:value={role}
								aria-label={m.users_field_role()}
								class="grid grid-cols-2 gap-3"
							>
								{#each roles as option (option.value)}
									<RadioGroup.Item
										value={option.value}
										class="flex items-start gap-3 rounded-md border border-border bg-surface-elevated px-4 py-3 text-left transition data-[state=checked]:border-[1.5px] data-[state=checked]:border-primary data-[state=checked]:bg-accent"
									>
										{#snippet children({ checked })}
											<span
												aria-hidden="true"
												class="mt-0.5 size-4 shrink-0 rounded-full border {checked
													? 'border-[5px] border-primary bg-surface'
													: 'border-border bg-surface-elevated'}"
											></span>
											<span class="flex flex-col">
												<span class="text-body-sm font-semibold">{option.label}</span>
												<span class="text-micro text-text-muted">{option.hint}</span>
											</span>
										{/snippet}
									</RadioGroup.Item>
								{/each}
							</RadioGroup.Root>
							<div class="flex justify-end gap-3 pt-2">
								<Button variant="ghost" onclick={() => (open = false)}>{m.common_cancel()}</Button>
								<Button type="submit" disabled={saving}>{m.users_create_submit()}</Button>
							</div>
						</form>
					</div>
				{/if}
			{/snippet}
		</Dialog.Content>
	</Dialog.Portal>
</Dialog.Root>
