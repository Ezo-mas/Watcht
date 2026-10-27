<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { tick } from 'svelte';
	import { ApiError } from '#lib/api/client.js';
	import type { Account } from '#lib/api/types.js';
	import { createUser, deleteUser, resetPassword } from '#lib/api/users.js';
	import Button from '#lib/components/Button.svelte';
	import Field from '#lib/components/Field.svelte';
	import InlineConfirm from '#lib/components/InlineConfirm.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import Panel from '#lib/components/Panel.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDate } from '#lib/format.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();
	const me = $derived(data.session.user?.id);

	let listHeading = $state<HTMLElement>();
	let note = $state('');

	// One row at a time has its reset form or delete question open under it.
	type Open = { user: Account; kind: 'reset' | 'delete' };
	let open = $state<Open | null>(null);
	let rowBusy = $state(false);
	let rowError = $state('');
	let newPassword = $state('');

	function toggle(user: Account, kind: Open['kind']) {
		if (open?.user.id === user.id && open.kind === kind) return close();
		open = { user, kind };
		rowError = '';
		newPassword = '';
		note = '';
	}

	// Closes the open row and puts focus back on the button that opened it.
	async function close() {
		const was = open;
		open = null;
		rowError = '';
		newPassword = '';
		await tick();
		if (was) document.getElementById(`${was.kind}-${was.user.id}`)?.focus();
	}

	async function rowFailed(e: unknown) {
		rowError = errorMessage(e);
		// Deleted meanwhile (another tab, another admin): refresh the list.
		if (e instanceof ApiError && e.code === 'not_found') await invalidate('app:users');
	}

	async function submitReset(event: SubmitEvent) {
		event.preventDefault();
		if (!open) return;
		const user = open.user;
		rowBusy = true;
		rowError = '';
		try {
			await resetPassword(user.id, newPassword);
		} catch (e) {
			await rowFailed(e);
			rowBusy = false;
			return;
		}
		rowBusy = false;
		await close();
		// API tokens keep working after a reset (maintainer decision); only
		// browser sessions end.
		note = `Password reset. ${user.username} has been signed out of every browser; their API tokens still work.`;
		await invalidate('app:users');
	}

	async function confirmDelete() {
		if (!open) return;
		const user = open.user;
		rowBusy = true;
		rowError = '';
		try {
			await deleteUser(user.id);
		} catch (e) {
			await rowFailed(e);
			rowBusy = false;
			return;
		}
		await invalidate('app:users');
		open = null;
		rowBusy = false;
		note = `Deleted ${user.username} and everything they tracked.`;
		listHeading?.focus();
	}

	function onRowKeydown(event: KeyboardEvent) {
		if (event.key === 'Escape' && !rowBusy) close();
	}

	// Add a user.

	let username = $state('');
	let password = $state('');
	let isAdmin = $state(false);
	let addError = $state('');
	let addNote = $state('');
	let addBusy = $state(false);

	async function submitAdd(event: SubmitEvent) {
		event.preventDefault();
		addBusy = true;
		addError = '';
		addNote = '';
		try {
			const user = await createUser(username, password, isAdmin);
			username = password = '';
			isAdmin = false;
			addNote = `Added ${user.username}${user.is_admin ? ' as an admin' : ''}. They can sign in now.`;
			await invalidate('app:users');
		} catch (e) {
			addError = errorMessage(e, { username_taken: 'That username is taken.' });
		} finally {
			addBusy = false;
		}
	}

	function focusOnMount(node: HTMLElement) {
		node.focus();
	}
</script>

<svelte:head>
	<title>Users · watcht</title>
</svelte:head>

<div class="flex flex-col gap-5">
	<div>
		<h1 class="text-xl font-semibold tracking-tight text-ink">Users</h1>
		<p class="mt-0.5 text-sm text-muted">
			Everyone with an account here. Each keeps their own history, ratings and watchlist.
		</p>
	</div>

	<div class="grid items-start gap-5 lg:grid-cols-[minmax(0,1fr)_22rem]">
		<Panel
			title="Accounts"
			intro="{data.users.length} {data.users.length === 1 ? 'account' : 'accounts'}"
			bind:headingRef={listHeading}
			flush
		>
			<Notice tone="success" message={note} class="mx-4 mt-4" />

			<table class="w-full text-sm">
				<thead class="max-sm:sr-only">
					<tr class="text-left text-xs text-muted">
						<th scope="col" class="px-4 pt-3 pb-1.5 font-medium">Username</th>
						<th scope="col" class="px-4 pt-3 pb-1.5 font-medium">Created</th>
						<th scope="col" class="px-4 pt-3 pb-1.5"><span class="sr-only">Actions</span></th>
					</tr>
				</thead>
				<tbody>
					{#each data.users as user (user.id)}
						{@const self = user.id === me}
						{@const isOpen = open?.user.id === user.id}
						<!-- On a phone each row stacks: name, then date and actions. -->
						<tr
							class={[
								'border-t border-line max-sm:flex max-sm:flex-wrap max-sm:items-center max-sm:gap-x-3 max-sm:gap-y-2 max-sm:px-4 max-sm:py-3 max-sm:first:border-t-0',
								isOpen && 'bg-canvas/60'
							]}
						>
							<th
								scope="row"
								class="px-4 py-2.5 text-left font-normal max-sm:w-full max-sm:p-0 sm:w-1/2 sm:max-w-0"
							>
								<span class="flex min-w-0 items-center gap-2">
									<span class="truncate font-medium text-ink" title={user.username}
										>{user.username}</span
									>
									{#if self}
										<span class="shrink-0 text-muted">(you)</span>
									{/if}
									{#if user.is_admin}
										<span
											class="shrink-0 rounded-sm bg-accent/15 px-1.5 py-px text-xs font-medium text-accent-text"
											>Admin</span
										>
									{/if}
								</span>
							</th>
							<td
								class="px-4 py-2.5 whitespace-nowrap text-muted max-sm:flex-1 max-sm:p-0 max-sm:text-xs"
							>
								<span class="sm:hidden">Created&nbsp;</span><time datetime={user.created_at}
									>{formatDate(user.created_at)}</time
								>
							</td>
							<td class="px-4 py-2 text-right max-sm:p-0">
								{#if !self}
									<span class="inline-flex gap-2">
										<Button
											variant="quiet"
											id="reset-{user.id}"
											aria-expanded={isOpen && open?.kind === 'reset'}
											aria-label="Reset password for {user.username}"
											onclick={() => toggle(user, 'reset')}>Reset password</Button
										>
										<Button
											variant="quiet"
											id="delete-{user.id}"
											aria-expanded={isOpen && open?.kind === 'delete'}
											aria-label="Delete {user.username}"
											onclick={() => toggle(user, 'delete')}>Delete</Button
										>
									</span>
								{/if}
							</td>
						</tr>
						{#if isOpen && open}
							<tr class="bg-canvas/60 max-sm:block">
								<td colspan="3" class="px-4 pt-1 pb-4 max-sm:block">
									{#if open.kind === 'delete'}
										<InlineConfirm
											confirmLabel="Delete {user.username}"
											busy={rowBusy}
											error={rowError}
											onconfirm={confirmDelete}
											oncancel={close}
										>
											Delete <strong class="font-semibold">{user.username}</strong>? Their watch
											history, ratings and watchlist go with the account. This can't be undone.
										</InlineConfirm>
									{:else}
										<!-- Escape from the field or its buttons cancels. -->
										<!-- svelte-ignore a11y_no_noninteractive_element_interactions -->
										<form
											onsubmit={submitReset}
											onkeydown={onRowKeydown}
											aria-label="Reset password for {user.username}"
											class="flex flex-col gap-3 rounded-md border border-line bg-surface p-3"
										>
											<div class="flex flex-col gap-3 sm:flex-row sm:items-start">
												<Field
													class="sm:w-80"
													label="New password for {user.username}"
													name="new-password"
													type="password"
													autocomplete="new-password"
													required
													minlength={8}
													hint="At least 8 characters. Signs them out everywhere."
													bind:value={newPassword}
													{@attach focusOnMount}
												/>
												<div class="flex items-center gap-2 sm:mt-6.5">
													<Button type="submit" busy={rowBusy}
														>{rowBusy ? 'Resetting…' : 'Reset password'}</Button
													>
													<Button variant="quiet" disabled={rowBusy} onclick={close}>Cancel</Button>
												</div>
											</div>
											<Notice tone="error" message={rowError} />
										</form>
									{/if}
								</td>
							</tr>
						{/if}
					{/each}
				</tbody>
			</table>
		</Panel>

		<Panel title="Add a user" intro="They sign in with these details and can change the password.">
			<form onsubmit={submitAdd} class="flex flex-col gap-4">
				<Field
					label="Username"
					name="new-username"
					autocomplete="off"
					autocapitalize="none"
					spellcheck="false"
					required
					maxlength={64}
					pattern="[A-Za-z0-9._\-]+"
					title="Letters, digits, '.', '_' or '-'"
					hint="Letters, digits, dots, hyphens and underscores."
					bind:value={username}
				/>
				<Field
					label="Password"
					name="new-user-password"
					type="password"
					autocomplete="new-password"
					required
					minlength={8}
					hint="At least 8 characters."
					bind:value={password}
				/>
				<label class="flex items-start gap-2.5">
					<input
						type="checkbox"
						bind:checked={isAdmin}
						aria-describedby="admin-hint"
						class="mt-0.5 size-4 shrink-0 accent-accent"
					/>
					<span class="flex flex-col gap-0.5">
						<span class="text-sm font-medium text-ink">Admin</span>
						<span id="admin-hint" class="text-xs text-muted"
							>Admins can add, reset and delete accounts.</span
						>
					</span>
				</label>
				<Notice tone="error" message={addError} />
				<div>
					<Button type="submit" busy={addBusy}>{addBusy ? 'Adding…' : 'Add user'}</Button>
				</div>
			</form>
			<Notice tone="success" message={addNote} class="mt-4" />
		</Panel>
	</div>
</div>
