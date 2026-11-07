<template>
	<div>
		<Teleport
			defer
			to="#trash-actions"
		>
			<XButton
				v-cy="'emptyTaskTrash'"
				variant="secondary"
				icon="trash-alt"
				danger
				:aria-disabled="total === 0 || undefined"
				@click="openEmptyModal()"
			>
				{{ $t('trash.empty') }}
			</XButton>
		</Teleport>

		<div
			ref="listElement"
			tabindex="-1"
		>
			<Card
				v-if="items.length > 0"
				:padding="false"
				class="has-overflow"
				:has-content="false"
				:loading="isFetching"
			>
				<ul class="trashed-tasks">
					<li
						v-for="task in items"
						:key="task.id"
						v-cy="'trashedTask'"
						:data-trash-row="task.id"
						class="trashed-task"
					>
						<SingleTaskInlineReadonly
							:task="task"
							show-project
						/>
						<span
							v-tooltip="formatDateLong(task.deleted_at)"
							class="trashed-task-meta"
						>
							{{ deletedLabel(task) }} ·
							{{ $t('trash.daysLeftShort', trashDaysLeft(task.deleted_at ?? '')) }}
						</span>
						<TrashRowActions
							:pending="isPending(task.id)"
							:can-delete-forever="task.can_delete_permanently"
							restore-cy="restoreTask"
							delete-forever-cy="deleteTaskForever"
							@restore="restore(task.id)"
							@deleteForever="confirmDeleteForever(task)"
						/>
					</li>
				</ul>
			</Card>
			<p
				v-else-if="showEmptyState"
				ref="emptyStateElement"
				tabindex="-1"
				class="has-text-centered has-text-grey is-italic"
			>
				{{ $t('trash.noTasks') }}
			</p>
		</div>

		<PaginationEmit
			v-if="totalPages > 1"
			:total-pages="totalPages"
			:current-page="currentPage"
			@pageChanged="currentPage = $event"
		/>

		<Modal
			:enabled="itemToDelete !== null"
			@close="cancelDeleteForever()"
			@submit="deleteForever()"
		>
			<template #header>
				<span>{{ $t('trash.deleteForeverModal.header') }}</span>
			</template>
			<template #text>
				<p class="tw:text-balance">
					{{ $t('trash.deleteForeverModal.text', {task: itemToDelete?.title}) }}
				</p>
			</template>
		</Modal>

		<EmptyTrashModal
			v-model:include-others="includeOthers"
			:enabled="showEmptyModal"
			:count="emptyCount"
			:texts="{text: 'trash.emptyModal.text', byOthers: 'trash.emptyModal.byOthers', includeOthers: 'trash.emptyModal.includeOthers'}"
			@close="closeEmptyModal()"
			@submit="emptyTrash()"
		/>
	</div>
</template>

<script setup lang="ts">
import {computed} from 'vue'
import {useQuery} from '@tanstack/vue-query'

import Card from '@/components/misc/Card.vue'
import Modal from '@/components/misc/Modal.vue'
import PaginationEmit from '@/components/misc/PaginationEmit.vue'
import SingleTaskInlineReadonly from '@/components/tasks/partials/SingleTaskInlineReadonly.vue'
import EmptyTrashModal from '@/components/trash/EmptyTrashModal.vue'
import TrashRowActions from '@/components/trash/TrashRowActions.vue'

import {
	taskTrashCountQuery,
	trashDaysLeft,
	trashedTasksQuery,
	useDeleteTaskPermanentlyMutation,
	useEmptyTaskTrashMutation,
	useRestoreTaskMutation,
} from '@/client/queries/trash'
import {useDeletedLabel} from '@/composables/useDeletedLabel'
import {useTrashList} from '@/composables/useTrashList'
import {formatDateLong} from '@/helpers/time/formatDate'

const deletedLabel = useDeletedLabel({
	unknown: 'trash.deleted',
	by: 'trash.deletedBy',
	byYou: 'trash.deletedByYou',
})

const {
	currentPage,
	items,
	total,
	totalPages,
	isFetching,
	showEmptyState,
	listElement,
	emptyStateElement,
	isPending,
	itemToDelete,
	showEmptyModal,
	emptyCount,
	includeOthers,
	restore,
	confirmDeleteForever,
	cancelDeleteForever,
	deleteForever,
	openEmptyModal,
	closeEmptyModal,
	emptyTrash,
} = useTrashList({
	query: page => useQuery(computed(() => trashedTasksQuery(page.value))),
	restore: useRestoreTaskMutation(),
	deleteForever: useDeleteTaskPermanentlyMutation(),
	count: enabled => useQuery(computed(() => ({...taskTrashCountQuery(), enabled: enabled.value}))),
	empty: useEmptyTaskTrashMutation(),
})

</script>

<style lang="scss" scoped>
// prime-tasks: Yuyu list (docs/DESIGN.md §4.8 in Yuyu): full-width rows with 1px separators.
.trashed-tasks {
	list-style: none;
	margin: 0;
	padding: 0;
}

.trashed-task {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: var(--yy-space-2, .5rem);
	padding: var(--yy-space-3, .75rem) var(--yy-space-4, 1rem);
	transition: background-color var(--yy-duration-hover, 180ms) var(--yy-ease, ease);

	& + & {
		border-block-start: 1px solid var(--yy-border, var(--grey-200));
	}

	&:hover {
		background-color: var(--yy-bg-subtle, var(--grey-100));
	}

	// Keep the hover fill inside the card's rounded corners.
	&:first-child {
		border-start-start-radius: calc(var(--yy-radius-lg, 16px) - 1px);
		border-start-end-radius: calc(var(--yy-radius-lg, 16px) - 1px);
	}

	&:last-child {
		border-end-start-radius: calc(var(--yy-radius-lg, 16px) - 1px);
		border-end-end-radius: calc(var(--yy-radius-lg, 16px) - 1px);
	}

	.task {
		cursor: default;
	}
}

.trashed-task-meta {
	margin-inline-start: auto;
	color: var(--yy-text-muted, var(--grey-500));
	font-size: .8125rem;
	white-space: nowrap;
}
</style>
