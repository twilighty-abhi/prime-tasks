<template>
	<Modal
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
// prime-tasks F4: confirm dialog for leaving a project shared with you directly.
import {computed} from 'vue'
import {useI18n} from 'vue-i18n'
import {useRoute, useRouter} from 'vue-router'

import {useTitle} from '@/composables/useTitle'
import {useGoBack} from '@/composables/useGoBack'
import {useProjects} from '@/composables/useProjects'
import {useLeaveProjectMutation} from '@/client/queries/projectShares'

const {t} = useI18n({useScope: 'global'})
const projectList = useProjects()
const leaveMutation = useLeaveProjectMutation()
const route = useRoute()
const router = useRouter()
const goBack = useGoBack()

const projectId = computed(() => Number(route.params.projectId))
const project = computed(() => projectList.projects[projectId.value])

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
