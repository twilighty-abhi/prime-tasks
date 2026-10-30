<template>
	<div class="share-dialog">
		<!-- prime-tasks F5: people and teams in one search, the role picked inline -->
		<div
			v-if="userIsAdmin"
			class="share-dialog__add"
		>
			<div class="share-dialog__search">
				<label
					class="is-sr-only"
					:for="searchId"
				>{{ $t('project.share.dialog.searchLabel') }}</label>
				<input
					:id="searchId"
					v-model="search"
					v-cy="'shareSearch'"
					type="search"
					class="input"
					autocomplete="off"
					role="combobox"
					:aria-expanded="candidates.length > 0"
					:aria-controls="resultsId"
					:placeholder="$t('project.share.dialog.searchPlaceholder')"
					@keydown.down.prevent="focusCandidate(0)"
				>
				<ul
					v-if="candidates.length > 0"
					:id="resultsId"
					ref="results"
					class="share-dialog__results"
					role="listbox"
				>
					<li
						v-for="(candidate, index) in candidates"
						:key="`${candidate.kind}-${candidate.id}`"
						role="option"
						:aria-selected="false"
					>
						<button
							type="button"
							class="share-dialog__result"
							:disabled="isMutating"
							@click="add(candidate)"
							@keydown.down.prevent="focusCandidate(index + 1)"
							@keydown.up.prevent="index === 0 ? focusSearch() : focusCandidate(index - 1)"
						>
							<User
								v-if="candidate.kind === 'user'"
								:user="candidate.user"
								:avatar-size="28"
								:show-username="true"
							/>
							<template v-else>
								<span
									class="share-dialog__initials"
									aria-hidden="true"
								>{{ initials(candidate.name) }}</span>
								<span>{{ candidate.name }}</span>
								<span class="tag">{{ $t('project.share.dialog.team') }}</span>
							</template>
						</button>
					</li>
				</ul>
			</div>
			<div class="select">
				<select
					v-model="newPermission"
					:aria-label="$t('project.share.dialog.roleLabel')"
				>
					<option :value="PERMISSIONS.READ">
						{{ $t('project.share.permission.read') }}
					</option>
					<option :value="PERMISSIONS.READ_WRITE">
						{{ $t('project.share.permission.readWrite') }}
					</option>
					<option :value="PERMISSIONS.ADMIN">
						{{ $t('project.share.permission.admin') }}
					</option>
				</select>
			</div>
		</div>

		<h3 class="share-dialog__heading">
			{{ $t('project.share.dialog.access') }}
		</h3>
		<Loading
			v-if="access.isPending.value"
			variant="small"
		/>
		<ul
			v-else
			class="share-dialog__list"
		>
			<li
				v-for="entry in entries"
				:key="entryKey(entry)"
				class="share-dialog__row"
			>
				<User
					v-if="entry.user"
					:user="entry.user"
					:avatar-size="32"
					:show-username="false"
					class="share-dialog__who"
				/>
				<span
					v-else
					class="share-dialog__who"
				>
					<span
						class="share-dialog__initials"
						aria-hidden="true"
					>{{ initials(entry.team?.name ?? '') }}</span>
					<span>{{ entry.team?.name }}</span>
				</span>
				<span class="share-dialog__source">
					{{ sourceLabel(entry) }}
					<template v-if="entry.user?.id === authStore.info?.id"> · {{ $t('project.share.userTeam.you') }}</template>
				</span>

				<template v-if="removing === entryKey(entry)">
					<span class="share-dialog__confirm">
						<span class="help">{{ $t('project.share.userTeam.removeCleanupText') }}</span>
						<XButton
							variant="secondary"
							class="is-small"
							@click="removing = null"
						>
							{{ $t('misc.cancel') }}
						</XButton>
						<XButton
							danger
							class="is-small"
							:loading="isMutating"
							@click="remove(entry)"
						>
							{{ $t('project.share.dialog.remove') }}
						</XButton>
					</span>
				</template>
				<template v-else>
					<div
						v-if="canManage(entry)"
						class="select is-small"
					>
						<select
							:value="entry.permission"
							:aria-label="$t('project.share.userTeam.permissionFor', {sharable: entryName(entry)})"
							:disabled="isMutating"
							@change="changePermission(entry, Number(($event.target as HTMLSelectElement).value))"
						>
							<option :value="PERMISSIONS.READ">
								{{ $t('project.share.permission.read') }}
							</option>
							<option :value="PERMISSIONS.READ_WRITE">
								{{ $t('project.share.permission.readWrite') }}
							</option>
							<option :value="PERMISSIONS.ADMIN">
								{{ $t('project.share.permission.admin') }}
							</option>
						</select>
					</div>
					<span
						v-else
						class="share-dialog__permission"
					>{{ permissionLabel(entry.permission) }}</span>
					<XButton
						v-if="canManage(entry)"
						variant="tertiary"
						class="is-small share-dialog__remove"
						:aria-label="$t('project.share.dialog.removeName', {name: entryName(entry)})"
						@click="removing = entryKey(entry)"
					>
						{{ $t('project.share.dialog.remove') }}
					</XButton>
				</template>
			</li>
		</ul>
	</div>
</template>

<script setup lang="ts">
// prime-tasks F5: one dialog for sharing a project with people and teams. Replaces the separate
// user and team forms; link sharing stays in ProjectSettingsShare, out of the way.
import {computed, nextTick, ref, useId, useTemplateRef, watch} from 'vue'
import {useQuery} from '@tanstack/vue-query'
import {useI18n} from 'vue-i18n'

import Loading from '@/components/misc/Loading.vue'
import User from '@/components/misc/User.vue'
import {
	projectAccessQuery,
	useCreateProjectTeamShareMutation,
	useCreateProjectUserShareMutation,
	useDeleteProjectTeamShareMutation,
	useDeleteProjectUserShareMutation,
	useUpdateProjectTeamShareMutation,
	useUpdateProjectUserShareMutation,
	type ProjectAccessEntry,
} from '@/client/queries/projectShares'
import {useUserSearch} from '@/composables/useUserSearch'
import {useTeamSearch} from '@/composables/useTeams'
import {PERMISSIONS, type Permission} from '@/constants/permissions'
import {getDisplayName} from '@/helpers/user'
import {useAuthStore} from '@/stores/auth'
import {useConfigStore} from '@/stores/config'
import type {User as IUser} from '@/client/generated'

const props = defineProps<{
	projectId: number
	userIsAdmin: boolean
}>()

const {t} = useI18n({useScope: 'global'})
const authStore = useAuthStore()
const configStore = useConfigStore()
const searchId = useId()
const resultsId = useId()

const access = useQuery(computed(() => ({...projectAccessQuery(props.projectId), enabled: props.projectId > 0})))
// The owner of a parent is usually the project's owner too; show each person once.
const entries = computed(() => {
	const seen = new Set<string>()
	return (access.data.value ?? []).filter(entry => {
		const key = entry.user ? `user-${entry.user.id}` : `team-${entry.team?.id}`
		if (entry.source === 'parent' && seen.has(key)) {
			return false
		}
		seen.add(key)
		return true
	})
})

const search = ref('')
const newPermission = ref<Permission>(PERMISSIONS.READ_WRITE)
const {users} = useUserSearch(search)
const {teams} = useTeamSearch({search, includePublic: () => configStore.public_teams_enabled, enabled: () => search.value !== ''})

type Candidate =
	| {kind: 'user', id: number, name: string, user: IUser & {id: number, username: string}}
	| {kind: 'team', id: number, name: string}

const directKeys = computed(() => new Set(entries.value
	.filter(entry => entry.source !== 'parent')
	.map(entry => entry.user ? `user-${entry.user.id}` : `team-${entry.team?.id}`)))

const candidates = computed<Candidate[]>(() => {
	if (search.value === '') {
		return []
	}
	const people = users.value.flatMap(user => user.id !== undefined && user.username
		? [{kind: 'user' as const, id: user.id, name: getDisplayName(user), user: {...user, id: user.id, username: user.username}}]
		: [])
	const groups = teams.value.flatMap(team => team.id !== undefined
		? [{kind: 'team' as const, id: team.id, name: team.name ?? ''}]
		: [])
	return [...people, ...groups].filter(candidate => !directKeys.value.has(`${candidate.kind}-${candidate.id}`))
})

const createUser = useCreateProjectUserShareMutation()
const createTeam = useCreateProjectTeamShareMutation()
const updateUser = useUpdateProjectUserShareMutation()
const updateTeam = useUpdateProjectTeamShareMutation()
const deleteUser = useDeleteProjectUserShareMutation()
const deleteTeam = useDeleteProjectTeamShareMutation()
const isMutating = computed(() => [createUser, createTeam, updateUser, updateTeam, deleteUser, deleteTeam]
	.some(mutation => mutation.isPending.value))

const removing = ref<string | null>(null)
watch(() => props.projectId, () => {
	search.value = ''
	removing.value = null
})

async function add(candidate: Candidate) {
	try {
		if (candidate.kind === 'user') {
			await createUser.mutateAsync({projectId: props.projectId, username: candidate.user.username, permission: newPermission.value})
		} else {
			await createTeam.mutateAsync({projectId: props.projectId, teamId: candidate.id, permission: newPermission.value})
		}
	} catch {
		return
	}
	search.value = ''
}

async function changePermission(entry: ProjectAccessEntry, permission: number) {
	const value = permission === PERMISSIONS.ADMIN || permission === PERMISSIONS.READ_WRITE ? permission : PERMISSIONS.READ
	try {
		if (entry.user?.username) {
			await updateUser.mutateAsync({projectId: props.projectId, username: entry.user.username, permission: value})
		} else if (entry.team?.id) {
			await updateTeam.mutateAsync({projectId: props.projectId, teamId: entry.team.id, permission: value})
		}
	} catch {
		// the mutation toasts; the select snaps back when the access list refetches
	}
}

async function remove(entry: ProjectAccessEntry) {
	try {
		if (entry.user?.username) {
			await deleteUser.mutateAsync({projectId: props.projectId, username: entry.user.username})
		} else if (entry.team?.id) {
			await deleteTeam.mutateAsync({projectId: props.projectId, teamId: entry.team.id})
		}
	} catch {
		return
	}
	removing.value = null
}

// Only this project's own shares are managed here; inherited ones belong to the parent.
function canManage(entry: ProjectAccessEntry) {
	return props.userIsAdmin && entry.source === 'direct'
}

function entryKey(entry: ProjectAccessEntry) {
	return `${entry.source}-${entry.kind}-${entry.user?.id ?? entry.team?.id}-${entry.via_project_id ?? 0}`
}

function entryName(entry: ProjectAccessEntry) {
	return entry.user ? getDisplayName(entry.user) : entry.team?.name ?? ''
}

function sourceLabel(entry: ProjectAccessEntry) {
	switch (entry.source) {
		case 'owner':
			return t('project.share.dialog.source.owner')
		case 'parent':
			return t('project.share.dialog.source.parent', {project: entry.via_project_title ?? ''})
		default:
			return entry.kind === 'team'
				? t('project.share.dialog.source.team')
				: t('project.share.dialog.source.direct')
	}
}

function permissionLabel(permission: number | undefined) {
	if (permission === PERMISSIONS.ADMIN) return t('project.share.permission.admin')
	if (permission === PERMISSIONS.READ_WRITE) return t('project.share.permission.readWrite')
	return t('project.share.permission.read')
}

function initials(name: string) {
	return name.split(/\s+/).filter(Boolean).slice(0, 2).map(part => part[0]!.toUpperCase()).join('')
}

const results = useTemplateRef<HTMLUListElement>('results')
async function focusCandidate(index: number) {
	await nextTick()
	const buttons = results.value?.querySelectorAll<HTMLButtonElement>('button')
	buttons?.[Math.min(index, buttons.length - 1)]?.focus()
}
function focusSearch() {
	document.getElementById(searchId)?.focus()
}
</script>

<style lang="scss" scoped>
.share-dialog__add {
	display: flex;
	gap: var(--yy-space-2);
	align-items: flex-start;
	margin-block-end: var(--yy-space-6);

	@media screen and (max-width: $tablet) {
		flex-direction: column;
		align-items: stretch;
	}
}

.share-dialog__search {
	position: relative;
	flex: 1;
}

.share-dialog__results {
	position: absolute;
	inset-inline: 0;
	inset-block-start: calc(100% + var(--yy-space-1));
	z-index: 10;
	margin: 0;
	padding: var(--yy-space-1);
	list-style: none;
	background: var(--yy-bg-canvas);
	border: 1px solid var(--yy-border);
	border-radius: var(--yy-radius-lg);
	box-shadow: var(--yy-shadow-float);
}

.share-dialog__result {
	display: flex;
	align-items: center;
	gap: var(--yy-space-2);
	inline-size: 100%;
	min-block-size: 40px;
	padding: var(--yy-space-1) var(--yy-space-2);
	border: 0;
	border-radius: var(--yy-radius-sm);
	background: transparent;
	color: var(--yy-text);
	font: inherit;
	text-align: start;
	cursor: pointer;

	&:hover,
	&:focus-visible {
		background: var(--yy-bg-emphasis);
	}
}

.share-dialog__heading {
	font-family: var(--yy-font-body);
	font-size: 14px !important;
	font-weight: 600 !important;
	color: var(--yy-text);
	margin-block-end: var(--yy-space-2);
}

.share-dialog__list {
	margin: 0;
	padding: 0;
	list-style: none;
}

.share-dialog__row {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: var(--yy-space-2) var(--yy-space-3);
	padding-block: var(--yy-space-3);
	border-block-end: 1px solid var(--yy-border);

	&:last-child {
		border-block-end: 0;
	}
}

.share-dialog__who {
	display: inline-flex;
	align-items: center;
	gap: var(--yy-space-2);
	flex: 1 1 12rem;
	min-inline-size: 0;
	color: var(--yy-text);
	font-weight: 600;
}

.share-dialog__initials {
	display: inline-flex;
	align-items: center;
	justify-content: center;
	flex-shrink: 0;
	inline-size: 32px;
	block-size: 32px;
	border-radius: var(--yy-radius-full);
	background: var(--yy-bg-muted);
	color: var(--yy-text-secondary);
	font-size: 12px;
	font-weight: 700;
}

.share-dialog__source,
.share-dialog__permission {
	color: var(--yy-text-muted);
	font-size: 13px;
}

.share-dialog__source {
	flex: 1 1 8rem;
}

.share-dialog__remove {
	color: var(--yy-text-muted);
}

.share-dialog__confirm {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: var(--yy-space-2);
	inline-size: 100%;
}
</style>
