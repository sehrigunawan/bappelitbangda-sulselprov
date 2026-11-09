<script lang="ts">
	import type { components } from '#lib/api/schema.d.ts';

	type Role = components['schemas']['Role'];

	interface Props {
		role: Role;
	}

	let { role }: Props = $props();

	const ROLE_LABELS: Record<Role, string> = {
		super_admin: 'Super Admin',
		humas_editor: 'Editor Humas',
		document_operator: 'Operator Dokumen',
		ppid_officer: 'Petugas PPID',
		leadership: 'Pimpinan'
	};
</script>

<span class="role-badge role-{role}">
	{ROLE_LABELS[role] ?? role}
</span>
