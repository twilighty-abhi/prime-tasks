<template>
	<div class="edit-assignees-wrapper">
		<Multiselect
			v-model="assignees"
			class="edit-assignees"
			:class="{'has-assignees': assignees.length > 0}"
			:loading="usersLoading"
			:placeholder="$t('task.assignee.placeholder')"
			:multiple="true"
			:search-results="foundUsers"
			:show-empty="true"
			label="name"
			:select-placeholder="$t('task.assignee.selectPlaceholder')"
			:autocomplete-enabled="false"
			@search="findUser"
			@select="addAssignee"
			@focus="preloadUsers"
		>
			<template #items="{items}">
				<AssigneeList
					:assignees="items"
					:disabled="disabled"
					can-remove
					@remove="removeAssignee"
				/>
			</template>
			<template #searchResult="{option: user}">
				<User
					:avatar-size="24"
					:show-username="true"
					:user="user"
				/>
				<span
					v-if="!hasProjectAccess(user)"
					class="tag not-in-project"
				>{{ $t('task.assignee.notInProject') }}</span>
			</template>
		</Multiselect>
		<!-- prime-tasks F2 -->
		<ShareAndAssignDialog
			:user="pendingShare"
			:loading="shareAndAssignMutation.isPending.value"
			@confirm="confirmShareAndAssign"
			@cancel="pendingShare = null"
		/>
		<p
			v-if="adminNeededFor"
			class="help admin-needed"
			role="status"
		>
			{{ $t('task.assignee.shareAndAssign.adminNeeded', {name: getDisplayName(adminNeededFor)}) }}
		</p>
	</div>
</template>

<script setup lang="ts">
import {ref, computed, watch, nextTick} from 'vue'
import {useI18n} from 'vue-i18n'

import User from '@/components/misc/User.vue'
import Multiselect from '@/components/input/Multiselect.vue'

import {useProjectUserSearch, useUserSearch} from '@/composables/useUserSearch'
import {success} from '@/message'
import {useAuthStore} from '@/stores/auth'
import {useConfigStore} from '@/stores/config'
import {useProjects} from '@/composables/useProjects'
import {useAddTaskAssigneeMutation, useRemoveTaskAssigneeMutation, useShareAndAssignTaskMutation} from '@/client/queries/taskMutations'
import {assigneeAction} from '@/helpers/assigneeAccess'
import ShareAndAssignDialog from '@/components/tasks/partials/ShareAndAssignDialog.vue'

import type {User as IUser} from '@/client/generated'
import {getDisplayName, type UserWithId} from '@/helpers/user'
import AssigneeList from '@/components/tasks/partials/AssigneeList.vue'

const props = withDefaults(defineProps<{
	modelValue: IUser[] | undefined,
	taskId: number,
	projectId: number,
	disabled?: boolean,
}>(), {
	disabled: false,
})

const emit = defineEmits<{
	'update:modelValue': [value: IUser[] | undefined],
}>()

const authStore = useAuthStore()
const addAssigneeMutation = useAddTaskAssigneeMutation()
const removeAssigneeMutation = useRemoveTaskAssigneeMutation()
const {t} = useI18n({useScope: 'global'})

const userSearch = ref('')
const searchEnabled = ref(false)
const {users: projectUserResults, isFetching: projectUsersLoading} = useProjectUserSearch(() => props.projectId, userSearch, searchEnabled)

// prime-tasks F2: with the org directory on (F1), search everyone, not just the project's members.
const configStore = useConfigStore()
const projectList = useProjects()
const orgDirectory = computed(() => configStore.org_directory === true)
const {users: directoryResults, isFetching: directoryLoading} = useUserSearch(() => orgDirectory.value ? userSearch.value : '')
const usersLoading = computed(() => projectUsersLoading.value || directoryLoading.value)
const projectMemberIds = computed(() => new Set(withIds(projectUserResults.value).map(user => user.id)))
const userResults = computed(() => [
	...projectUserResults.value,
	...directoryResults.value.filter(user => user.id !== undefined && !projectMemberIds.value.has(user.id)),
])

function hasProjectAccess(user: UserWithId) {
	return projectMemberIds.value.has(user.id)
}

const shareAndAssignMutation = useShareAndAssignTaskMutation()
const pendingShare = ref<UserWithId | null>(null)
const adminNeededFor = ref<UserWithId | null>(null)
const assignees = ref<UserWithId[]>([])
let isAdding = false

function preloadUsers() {
	searchEnabled.value = true
}

watch(
	() => props.modelValue,
	(value) => {
		assignees.value = withIds(value ?? [])
	},
	{
		immediate: true,
		deep: true,
	},
)

async function addAssignee(user: UserWithId) {
	if (isAdding) {
		return
	}

	adminNeededFor.value = null
	const action = assigneeAction(hasProjectAccess(user), projectList.projects[props.projectId]?.max_permission)
	if (action !== 'assign') {
		// The picker already shows them as picked; undo that until the share is decided.
		assignees.value = assignees.value.filter(a => a.id !== user.id)
		if (action === 'share-assign') {
			pendingShare.value = user
		} else {
			adminNeededFor.value = user
		}
		return
	}

	try {
		nextTick(() => isAdding = true)

		await addAssigneeMutation.mutateAsync({user: user, taskId: props.taskId})
		emit('update:modelValue', assignees.value)
		success({message: t('task.assignee.assignSuccess')})
	} finally {
		nextTick(() => isAdding = false)
	}
}

async function confirmShareAndAssign() {
	const user = pendingShare.value
	if (!user) {
		return
	}
	try {
		await shareAndAssignMutation.mutateAsync({user, taskId: props.taskId})
	} catch {
		return
	}
	pendingShare.value = null
	assignees.value = [...assignees.value.filter(a => a.id !== user.id), user]
	emit('update:modelValue', assignees.value)
	success({message: t('task.assignee.shareAndAssign.success', {name: getDisplayName(user)})})
}

async function removeAssignee(user: UserWithId) {
	await removeAssigneeMutation.mutateAsync({user: user, taskId: props.taskId})

	assignees.value = assignees.value.filter(a => a.id !== user.id)
	emit('update:modelValue', assignees.value)
	success({message: t('task.assignee.unassignSuccess')})
}

function findUser(query = '') {
	userSearch.value = query
	searchEnabled.value = true
}

function withIds(users: IUser[]): UserWithId[] {
	return users.flatMap(user => user.id === undefined ? [] : [{...user, id: user.id}])
}

const foundUsers = computed(() => withIds(userResults.value)
	.filter(({id}) => !assignees.value.some(assignee => assignee.id === id))
	.map(user => ({...user, name: getDisplayName(user)}))
	.sort((a, b) => {
		if (a.id === authStore.info?.id) return -1
		if (b.id === authStore.info?.id) return 1
		return a.name.localeCompare(b.name)
	}),
)
</script>

<style lang="scss">
.edit-assignees.has-assignees.multiselect .input {
	padding-inline-start: 0;
}
</style>

<style lang="scss" scoped>
.not-in-project {
	margin-inline-start: var(--yy-space-2);
}

.admin-needed {
	margin-block-start: var(--yy-space-2);
}
</style>
