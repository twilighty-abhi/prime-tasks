<template>
	<DropdownItem
		v-if="!isOwner"
		v-cy="'leaveProject'"
		:to="{ name: 'project.settings.leave', params: { projectId } }"
		icon="sign-out-alt"
		class="has-text-danger"
	>
		{{ $t('menu.leave') }}
	</DropdownItem>
</template>

<script setup lang="ts">
// prime-tasks F4: shown to everyone except the owner. The leave dialog itself works out whether the
// access is a direct share that can be left, or comes from a team or a parent project.
import {computed} from 'vue'

import DropdownItem from '@/components/misc/DropdownItem.vue'
import {useAuthStore} from '@/stores/auth'

const props = defineProps<{
	projectId: number
	ownerId?: number
}>()

const authStore = useAuthStore()
const isOwner = computed(() => props.ownerId !== undefined && props.ownerId === authStore.info?.id)
</script>
