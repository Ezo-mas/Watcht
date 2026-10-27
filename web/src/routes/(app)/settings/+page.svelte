<script lang="ts">
	import { invalidate } from '$app/navigation';
	import { tick } from 'svelte';
	import { changePassword, createToken, revokeToken } from '#lib/api/account.js';
	import { ApiError } from '#lib/api/client.js';
	import type { APIToken } from '#lib/api/types.js';
	import Button from '#lib/components/Button.svelte';
	import Field from '#lib/components/Field.svelte';
	import InlineConfirm from '#lib/components/InlineConfirm.svelte';
	import Notice from '#lib/components/Notice.svelte';
	import Panel from '#lib/components/Panel.svelte';
	import { errorMessage } from '#lib/errors.js';
	import { formatDate } from '#lib/format.js';
	import type { PageProps } from './$types';

	let { data }: PageProps = $props();

	const rateLimited = 'Too many attempts; try again later.';

	// Password.

	let current = $state('');
	let next = $state('');
	let confirm = $state('');
	let pwError = $state('');
	let pwNote = $state('');
	let pwBusy = $state(false);

	// The server checks length too; checking the match here gives the message sooner.
	const mismatch = $derived(confirm !== '' && confirm !== next);

	async function submitPassword(event: SubmitEvent) {
		event.preventDefault();
		pwNote = '';
		if (next !== confirm) {
			pwError = "The new passwords don't match.";
			return;
		}
		pwBusy = true;
		pwError = '';
		try {
			await changePassword(current, next);
			current = next = confirm = '';
			pwNote = 'Password changed. Your other devices have been signed out.';
		} catch (e) {
			pwError = errorMessage(e, {
				invalid_credentials: 'The current password is wrong.',
				rate_limited: rateLimited
			});
		} finally {
			pwBusy = false;
		}
	}

	// API tokens.

	let tokensHeading = $state<HTMLElement>();
	let tokenNote = $state('');

	let confirming = $state<APIToken | null>(null);
	let revokeBusy = $state(false);
	let revokeError = $state('');

	function askRevoke(token: APIToken) {
		if (revokeBusy) return; // one revoke at a time, so feedback lands on the right row
		if (confirming?.id === token.id) return cancelRevoke();
		confirming = token;
		revokeError = '';
		tokenNote = '';
	}

	async function cancelRevoke() {
		const id = confirming?.id;
		confirming = null;
		revokeError = '';
		await tick();
		if (id !== undefined) document.getElementById(`revoke-${id}`)?.focus();
	}

	async function revoke() {
		if (!confirming) return;
		const token = confirming;
		revokeBusy = true;
		revokeError = '';
		try {
			await revokeToken(token.id);
		} catch (e) {
			revokeBusy = false;
			if (e instanceof ApiError && e.code === 'not_found') {
				// Gone already (revoked in another tab): the row is about to
				// disappear, so say so outside it.
				confirming = null;
				tokenNote = `“${token.name}” was already revoked.`;
				await invalidate('app:tokens');
				tokensHeading?.focus();
				return;
			}
			revokeError = errorMessage(e);
			return;
		}
		await invalidate('app:tokens');
		confirming = null;
		revokeBusy = false;
		tokenNote = `Revoked “${token.name}”. Anything using it has lost access.`;
		tokensHeading?.focus();
	}

	let tokenName = $state('');
	let tokenPassword = $state('');
	let createError = $state('');
	let createBusy = $state(false);
	let nameInput = $state<HTMLInputElement>();

	// The new token's secret, shown once. Done sets it back to null, and
	// nothing else keeps a copy.
	let created = $state<{ name: string; secret: string } | null>(null);
	let copyNote = $state('');
	let secretCode = $state<HTMLElement>();

	async function submitToken(event: SubmitEvent) {
		event.preventDefault();
		createBusy = true;
		createError = '';
		tokenNote = '';
		try {
			const res = await createToken(tokenName, tokenPassword);
			created = { name: res.token.name, secret: res.secret };
			copyNote = '';
			tokenName = '';
			// The list refreshes on Done, not now: if the refresh failed, the
			// error page would replace this one and the secret, which can't be
			// shown again, would be lost.
		} catch (e) {
			createError = errorMessage(e, {
				invalid_credentials: 'The password is wrong.',
				name_taken: 'You already have a token with that name.',
				rate_limited: rateLimited
			});
		} finally {
			// Never keep the password around, whatever happened.
			tokenPassword = '';
			createBusy = false;
		}
	}

	async function copySecret() {
		if (!created) return;
		try {
			// Only available on https or localhost.
			await navigator.clipboard.writeText(created.secret);
			copyNote = 'Copied to the clipboard.';
		} catch {
			if (secretCode) getSelection()?.selectAllChildren(secretCode);
			copyNote = "Couldn't copy here: the token is selected, so press Ctrl+C or Cmd+C.";
		}
	}

	async function done() {
		created = null;
		copyNote = '';
		getSelection()?.removeAllRanges();
		await invalidate('app:tokens');
		await tick();
		nameInput?.focus();
	}

	function focusOnMount(node: HTMLElement) {
		node.focus();
	}
</script>

<svelte:head>
	<title>Settings · watcht</title>
</svelte:head>

<div class="flex flex-col gap-5">
	<div>
		<h1 class="text-xl font-semibold tracking-tight text-ink">Settings</h1>
		<p class="mt-0.5 text-sm text-muted">
			Signed in as <span class="font-medium text-ink">{data.session.user?.username}</span>{data
				.session.user?.is_admin
				? ', an admin.'
				: '.'}
		</p>
	</div>

	<div class="grid items-start gap-5 lg:grid-cols-[22rem_minmax(0,1fr)]">
		<Panel title="Password" intro="Changing it signs you out on your other devices.">
			<form onsubmit={submitPassword} class="flex flex-col gap-4">
				<!-- Lets password managers match the password to this account. -->
				<input
					type="text"
					name="username"
					autocomplete="username"
					value={data.session.user?.username ?? ''}
					readonly
					hidden
				/>
				<Field
					label="Current password"
					name="current-password"
					type="password"
					autocomplete="current-password"
					required
					bind:value={current}
				/>
				<Field
					label="New password"
					name="new-password"
					type="password"
					autocomplete="new-password"
					required
					minlength={8}
					hint="At least 8 characters."
					bind:value={next}
				/>
				<Field
					label="Confirm new password"
					name="confirm-password"
					type="password"
					autocomplete="new-password"
					required
					error={mismatch ? "The passwords don't match." : undefined}
					bind:value={confirm}
				/>
				<Notice tone="error" message={pwError} />
				<div>
					<Button type="submit" busy={pwBusy}>{pwBusy ? 'Changing…' : 'Change password'}</Button>
				</div>
			</form>
			<Notice tone="success" message={pwNote} class="mt-4" />
		</Panel>

		<Panel
			title="API tokens"
			intro="Tokens let scripts and media servers use watcht's API, and they keep working after a password change. Revoke one you no longer use."
			bind:headingRef={tokensHeading}
			flush
		>
			<Notice tone="success" message={tokenNote} class="mx-4 mt-4" />

			{#if data.tokens.length === 0 && !created}
				<p class="px-4 py-4 text-sm text-muted">
					No tokens yet. Create one below for each script or server, so you can revoke them one at a
					time.
				</p>
			{:else}
				<ul class="divide-y divide-line">
					{#each data.tokens as token (token.id)}
						<li class="px-4 py-2.5">
							<div class="flex items-center gap-3">
								<div class="min-w-0 flex-1">
									<p class="truncate text-sm font-medium text-ink" title={token.name}>
										{token.name}
									</p>
									<p class="text-xs text-muted">
										Created <time datetime={token.created_at}>{formatDate(token.created_at)}</time>
									</p>
								</div>
								<Button
									variant="quiet"
									id="revoke-{token.id}"
									aria-expanded={confirming?.id === token.id}
									aria-label="Revoke {token.name}"
									onclick={() => askRevoke(token)}
								>
									Revoke
								</Button>
							</div>
							{#if confirming?.id === token.id}
								<InlineConfirm
									class="mt-2.5"
									confirmLabel="Revoke token"
									busy={revokeBusy}
									error={revokeError}
									onconfirm={revoke}
									oncancel={cancelRevoke}
								>
									Revoke <strong class="font-semibold">“{token.name}”</strong>? Anything using it
									loses access straight away. This can't be undone.
								</InlineConfirm>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}

			<div class="border-t border-line bg-canvas/50 px-4 py-4">
				{#if created}
					<!--
						Focus lands here so the warning is read first. The amber frame
						already marks it, so it skips the usual focus ring.
					-->
					<div
						tabindex="-1"
						role="group"
						aria-labelledby="secret-title"
						aria-describedby="secret-warning"
						{@attach focusOnMount}
						class="rounded-md border border-l-4 border-accent bg-surface p-4 focus:outline-none"
					>
						<h3 id="secret-title" class="text-sm font-semibold text-ink">
							Your new token “{created.name}”
						</h3>
						<p id="secret-warning" class="mt-0.5 text-sm text-accent-text">
							Copy it now. watcht doesn't keep a copy, so it won't be shown again.
						</p>
						<div class="mt-3 flex flex-col gap-2 sm:flex-row sm:items-stretch">
							<code
								bind:this={secretCode}
								class="min-w-0 flex-1 rounded-md border border-line-strong bg-field px-3 py-2 font-mono text-sm break-all text-ink select-all"
								>{created.secret}</code
							>
							<Button variant="quiet" class="sm:h-auto" onclick={copySecret}>Copy</Button>
						</div>
						<div role="status" class="mt-1.5 min-h-4 text-xs text-muted">{copyNote}</div>
						<p class="mt-1 text-xs text-muted">
							Send it in a header: <code class="font-mono whitespace-nowrap text-ink"
								>Authorization: Bearer &lt;token&gt;</code
							>
						</p>
						<div class="mt-3">
							<Button onclick={done}>Done</Button>
						</div>
					</div>
				{:else}
					<form onsubmit={submitToken} class="flex flex-col gap-3">
						<h3 class="text-sm font-semibold text-ink">Create a token</h3>
						<div class="grid max-w-2xl gap-3 sm:grid-cols-2">
							<Field
								label="Name"
								name="token-name"
								autocomplete="off"
								required
								maxlength={64}
								hint="What will use it, e.g. “Jellyfin”."
								bind:value={tokenName}
								{@attach (node: HTMLElement) => {
									nameInput = node as HTMLInputElement;
								}}
							/>
							<Field
								label="Your password"
								name="token-password"
								type="password"
								autocomplete="current-password"
								required
								hint="Needed to create a token."
								bind:value={tokenPassword}
							/>
						</div>
						<Notice tone="error" message={createError} />
						<div>
							<Button type="submit" busy={createBusy}
								>{createBusy ? 'Creating…' : 'Create token'}</Button
							>
						</div>
					</form>
				{/if}
			</div>
		</Panel>
	</div>
</div>
