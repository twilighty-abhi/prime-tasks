<template>
	<Modal
		v-if="blocker"
		@close="goBack()"
	>
		<div class="modal-header">
			{{ $t('project.leave.blockedHeader') }}
		</div>
		<div class="content">
			<p>
				{{ blocker.kind === 'team'
					? $t('project.leave.teamOnly', {team: blocker.teamName})
					: $t('project.leave.parentOnly', {project: blocker.projectTitle}) }}
			</p>
		</div>
		<div class="actions">
			<XButton
				variant="tertiary"
				@click="goBack()"
			>
				{{ $t('misc.close') }}
			</XButton>
			<XButton
				v-if="blocker.kind === 'team'"
				variant="primary"
				:shadow="false"
				:to="{name: 'teams.edit', params: {id: blocker.teamId}}"
			>
				{{ $t('project.leave.openTeam', {team: blocker.teamName}) }}
			</XButton>
			<XButton
				v-else
				variant="primary"
				:shadow="false"
				:to="{name: 'project.index', params: {projectId: blocker.projectId}}"
			>
				{{ $t('project.leave.openParent', {project: blocker.projectTitle}) }}
			</XButton>
		</div>
	</Modal>
	<Modal
		v-else-if="!shares.isPending.value"
		@close="goBack()"
		@submit="leaveProject()"
	>
		<template #header>
			<span>{{ $t('project.leave.header') }}</span>
		</template>

		<template #text>
			<p>
				{{ $t('project.leave.text', {project: project?.title ?? ''}) }}
			</p>
		</template>
	</Modal>
</template>

<script setup lang="ts">
// prime-tasks F4: leave a project shared with you directly. When the access comes only from a team
// or a parent project, there is nothing to leave here, so the dialog says where it comes from instead.
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'
import {useQuery} from '@tanstack/vue-query'

import {useTitle} from '@/composables/useTitle'
import {useGoBack} from '@/composables/useGoBack'
import {useProjects} from '@/composables/useProjects'
import {projectUserSharesQuery, useLeaveProjectMutation} from '@/client/queries/projectShares'
import {leaveBlocker} from '@/helpers/projectSharing'
import {useAuthStore} from '@/stores/auth'

const {t} = useI18n({useScope: 'global'})
const projectList = useProjects()
const leaveMutation = useLeaveProjectMutation()
const authStore = useAuthStore()
const route = useRoute()
const router = useRouter()
const goBack = useGoBack()

const projectId = computed(() => Number(route.params.projectId))
const project = computed(() => projectList.projects[projectId.value])

const shares = useQuery(computed(() => ({
	...projectUserSharesQuery(projectId.value),
	enabled: projectId.value > 0,
	meta: {handlesError: true},
})))
const hasDirectShare = computed(() => (shares.data.value ?? []).some(share => share.username === authStore.info?.username))
const blocker = computed(() => {
	if (shares.isPending.value || !project.value) {
		return null
	}
	return leaveBlocker(project.value, hasDirectShare.value)
})

useTitle(() => t('project.leave.title', {project: project.value?.title ?? ''}))

async function leaveProject() {
	try {
		await leaveMutation.mutateAsync(projectId.value)
	} catch {
		return
	}
	router.push({name: 'home'})
}
</script>
