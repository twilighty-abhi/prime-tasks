<template>
	<DropdownItem
		v-if="hasDirectShare"
		v-cy="'leaveProject'"
		:to="{ name: 'project.settings.leave', params: { projectId } }"
		icon="sign-out-alt"
		class="has-text-danger"
	>
		{{ $t('menu.leave') }}
	</DropdownItem>
</template>

<script setup lang="ts">
// prime-tasks F4: shown only when the current user has a direct share they can remove.
// Mounted inside the project menu, so the share list loads only once the menu is used.
import {computed} from 'vue'
import {useQuery} from '@tanstack/vue-query'

import DropdownItem from '@/components/misc/DropdownItem.vue'
import {projectUserSharesQuery} from '@/client/queries/projectShares'
import {useAuthStore} from '@/stores/auth'

const props = defineProps<{
	projectId: number
	ownerId?: number
}>()

const authStore = useAuthStore()
const isOwner = computed(() => props.ownerId !== undefined && props.ownerId === authStore.info?.id)

const shares = useQuery(computed(() => ({
	...projectUserSharesQuery(props.projectId),
	enabled: !isOwner.value && props.projectId > 0,
	meta: {handlesError: true},
})))

const hasDirectShare = computed(() => !isOwner.value &&
	(shares.data.value ?? []).some(share => share.username === authStore.info?.username))
</script>
