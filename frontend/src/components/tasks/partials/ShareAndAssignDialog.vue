<template>
	<Modal
		v-if="user"
		@close="emit('cancel')"
		@submit="emit('confirm')"
	>
		<div
			:id="headerId"
			class="modal-header"
		>
			{{ $t('task.assignee.shareAndAssign.header') }}
		</div>
		<div class="content">
			<p>{{ $t('task.assignee.shareAndAssign.text', {name: getDisplayName(user)}) }}</p>
		</div>
		<div class="actions">
			<XButton
				variant="secondary"
				@click="emit('cancel')"
			>
				{{ $t('misc.cancel') }}
			</XButton>
			<XButton
				v-cy="'shareAndAssignConfirm'"
				variant="primary"
				:loading="loading"
				@click="emit('confirm')"
			>
				{{ $t('task.assignee.shareAndAssign.confirm') }}
			</XButton>
		</div>
	</Modal>
</template>

<script setup lang="ts">
// prime-tasks F2: "Give <name> read & write on this project and assign?"
import {useId} from 'vue'

import Modal from '@/components/misc/Modal.vue'
import {getDisplayName, type UserWithId} from '@/helpers/user'

defineProps<{
	user: UserWithId | null
	loading?: boolean
}>()

const emit = defineEmits<{
	confirm: []
	cancel: []
}>()

const headerId = useId()
</script>
