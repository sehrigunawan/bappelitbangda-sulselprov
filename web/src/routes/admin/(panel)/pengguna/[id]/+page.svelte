<script lang="ts">
	import { createApiClient } from '#lib/http/client.ts';
	import type { components } from '#lib/api/schema.d.ts';
	import StepUpDialog from '#lib/admin/StepUpDialog.svelte';

	type Role = components['schemas']['Role'];
	type User = components['schemas']['User'];
	type SessionState = components['schemas']['SessionState'];

	interface Props {
		data: {
			user?: User;
			session?: SessionState;
		};
		params: {
			id: string;
		};
	}

	let { data, params }: Props = $props();

	const user = $derived(data.user);
	const session = $derived(data.session);
	let fullName = $state('');
	let selectedRoles = $state<Role[]>([]);
	let active = $state(true);

	$effect(() => {
		if (data.user) {
			fullName = data.user.fullName;
			selectedRoles = [...data.user.roles];
			active = data.user.active;
		}
	});

	let loading = $state(false);
	let message = $state('');
	let errorMessage = $state('');
	let resetPasswordResult = $state<string | null>(null);

	// Step-up state
	let stepUpOpen = $state(false);
	let pendingAction = $state<'password' | 'totp' | null>(null);

	const AVAILABLE_ROLES: { role: Role; label: string }[] = [
		{ role: 'super_admin', label: 'Super Admin' },
		{ role: 'humas_editor', label: 'Editor Humas' },
		{ role: 'document_operator', label: 'Operator Dokumen' },
		{ role: 'ppid_officer', label: 'Petugas PPID' },
		{ role: 'leadership', label: 'Pimpinan' }
	];

	function toggleRole(role: Role) {
		if (selectedRoles.includes(role)) {
			if (selectedRoles.length > 1) {
				selectedRoles = selectedRoles.filter((r) => r !== role);
			}
		} else {
			selectedRoles = [...selectedRoles, role];
		}
	}

	async function handleSave(e: SubmitEvent) {
		e.preventDefault();
		loading = true;
		message = '';
		errorMessage = '';

		try {
			const client = createApiClient({ baseUrl: window.location.origin });
			const { data: updated, response } = await client.PUT('/api/v1/admin/auth/users/{userId}', {
				params: { path: { userId: Number(params.id) } },
				body: {
					fullName,
					roles: selectedRoles,
					active
				}
			});

			if (response.ok && updated) {
				message = 'Perubahan berhasil disimpan.';
			} else if (response.status === 409) {
				errorMessage = 'Konflik: Perubahan tidak diizinkan.';
			} else {
				errorMessage = 'Terjadi kesalahan. Coba lagi.';
			}
		} catch {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			loading = false;
		}
	}
	function hasActiveStepUp(): boolean {
		const expiresAt = session?.stepUpExpiresAt;
		return Boolean(expiresAt && new Date(expiresAt).getTime() > Date.now());
	}

	function requestResetPassword() {
		pendingAction = 'password';
		if (hasActiveStepUp()) {
			executeStepUpAction();
		} else {
			stepUpOpen = true;
		}
	}

	function requestResetTotp() {
		pendingAction = 'totp';
		if (hasActiveStepUp()) {
			executeStepUpAction();
		} else {
			stepUpOpen = true;
		}
	}
	async function executeStepUpAction() {
		stepUpOpen = false;
		loading = true;
		errorMessage = '';
		message = '';

		const client = createApiClient({ baseUrl: window.location.origin });

		try {
			if (pendingAction === 'password') {
				const { data: res, response } = await client.POST(
					'/api/v1/admin/auth/users/{userId}/password-reset',
					{ params: { path: { userId: Number(params.id) } } }
				);
				if (response.ok && res) {
					resetPasswordResult = res.temporaryPassword;
				} else if (response.status === 403) {
					stepUpOpen = true;
				} else {
					errorMessage = 'Terjadi kesalahan. Coba lagi.';
				}
			} else if (pendingAction === 'totp') {
				const { response } = await client.DELETE('/api/v1/admin/auth/users/{userId}/totp', {
					params: { path: { userId: Number(params.id) } }
				});
				if (response.ok) {
					message = 'Autentikator berhasil diatur ulang.';
				} else if (response.status === 403) {
					stepUpOpen = true;
				} else {
					errorMessage = 'Terjadi kesalahan. Coba lagi.';
				}
			}
		} catch {
			errorMessage = 'Terjadi kesalahan. Coba lagi.';
		} finally {
			loading = false;
			pendingAction = null;
		}
	}
</script>

<div class="user-detail-page">
	<h2>Detail Pengguna</h2>

	{#if resetPasswordResult}
		<div class="success-banner">
			Kata sandi sementara: {resetPasswordResult}. Berikan langsung kepada pengguna; kata sandi ini
			tidak akan ditampilkan lagi.
		</div>
	{/if}

	{#if message}
		<div class="success-alert">{message}</div>
	{/if}

	{#if errorMessage}
		<div class="error-alert">{errorMessage}</div>
	{/if}

	{#if user}
		<div class="user-summary">
			<h3>{user.fullName}</h3>
			<span class="username-tag">@{user.username}</span>
		</div>

		<form onsubmit={handleSave}>
			<div class="form-group">
				<label>
					Nama pengguna (tidak dapat diubah)
					<input type="text" value={user.username} disabled />
				</label>
			</div>

			<div class="form-group">
				<label>
					Nama lengkap
					<input type="text" bind:value={fullName} required disabled={loading} />
				</label>
			</div>

			<fieldset class="roles-fieldset">
				<legend>Peran</legend>
				{#each AVAILABLE_ROLES as { role, label } (role)}
					<label class="role-checkbox">
						<input
							type="checkbox"
							value={role}
							checked={selectedRoles.includes(role)}
							onchange={() => toggleRole(role)}
							disabled={loading}
						/>
						{label}
					</label>
				{/each}
			</fieldset>

			<div class="form-group">
				<label class="status-label">
					<span>Status</span>
					<select bind:value={active} disabled={loading}>
						<option value={true}>Aktif</option>
						<option value={false}>Nonaktif</option>
					</select>
				</label>
			</div>

			<div class="form-actions">
				<button type="submit" disabled={loading}>
					{loading ? 'Memuat...' : 'Simpan'}
				</button>
			</div>
		</form>

		<hr class="section-divider" />

		<div class="danger-zone">
			<h3>Tindakan Administratif</h3>
			<div class="action-buttons">
				<button type="button" class="btn-warning" onclick={requestResetPassword} disabled={loading}>
					Atur Ulang Kata Sandi
				</button>
				<button type="button" class="btn-danger" onclick={requestResetTotp} disabled={loading}>
					Atur Ulang Autentikator
				</button>
			</div>
		</div>
	{/if}

	<StepUpDialog
		open={stepUpOpen}
		client={createApiClient({ baseUrl: window.location.origin })}
		onverified={executeStepUpAction}
		oncancel={() => {
			stepUpOpen = false;
			pendingAction = null;
		}}
	/>
</div>
