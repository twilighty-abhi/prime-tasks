<template>
	<CreateEdit
		:title="$t('project.share.header')"
		:has-primary-action="false"
	>
		<ShareDialog
			v-if="project"
			:project-id="project.id"
			:user-is-admin="userIsAdmin"
		/>

		<!-- prime-tasks F5: link sharing still works, but out of the way (it's off in deploy/config.yml) -->
		<details
			v-if="configStore.link_sharing_enabled && userIsAdmin"
			class="link-sharing mbs-4"
		>
			<summary>{{ $t('project.share.links.title') }}</summary>
			<LinkSharing
				:project-id="projectId"
				class="mbs-4"
			/>
		</details>
	</CreateEdit>
</template>


<script lang="ts" setup>
import {computed, watchEffect} from 'vue'
import {useQuery} from '@tanstack/vue-query'
import {useRoute} from 'vue-router'
import {useI18n} from 'vue-i18n'
import {useTitle} from '@vueuse/core'

import {PERMISSIONS} from '@/constants/permissions'

import CreateEdit from '@/components/misc/CreateEdit.vue'
import LinkSharing from '@/components/sharing/LinkSharing.vue'
import ShareDialog from '@/components/sharing/ShareDialog.vue'

import {useBaseStore} from '@/stores/base'
import {useConfigStore} from '@/stores/config'
import {projectQuery} from '@/client/queries/projects'

defineOptions({name: 'ProjectSettingShare'})

const {t} = useI18n({useScope: 'global'})

const route = useRoute()
const projectId = computed(() => Number(route.params.projectId))
const {data: project} = useQuery(computed(() => ({...projectQuery(projectId.value), enabled: projectId.value > 0})))
const title = computed(() => project.value?.title
	? t('project.share.title', {project: project.value.title})
	: '',
)
useTitle(title)

const configStore = useConfigStore()

const userIsAdmin = computed(() => project.value?.max_permission === PERMISSIONS.ADMIN)

const baseStore = useBaseStore()
watchEffect(() => {
	if (project.value) baseStore.setCurrentProject(project.value)
})
</script>

<style lang="scss" scoped>
.link-sharing > summary {
	cursor: pointer;
	color: var(--yy-text-secondary);
	font-weight: 600;
}
</style>
