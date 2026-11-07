<template>
	<div
		ref="card"
		class="task loader-container draggable"
		:class="{
			'is-loading': showLoading,
			'draggable': !loadingInternal,
			'kanban-card--done': task.done,
			'has-cover': coverImageBlobUrl,
		}"
		:data-task-id="task.id"
		:data-project-id="task.project_id"
		:data-is-overdue="isOverdue || undefined"
		@click.exact="openTaskDetail()"
		@click.ctrl="() => toggleTaskDone(task)"
		@click.meta="() => toggleTaskDone(task)"
	>
		<span
			v-if="color"
			class="kanban-card__band"
			:style="{'background-color': color}"
			aria-hidden="true"
		/>
		<img
			v-if="coverImageBlobUrl"
			:src="coverImageBlobUrl"
			alt=""
			class="kanban-card__cover"
		>
		<div class="kanban-card__body">
			<!-- prime-tasks: compact card: the Done chip sits beside the title, the identifier leads the footer -->
			<div class="kanban-card__heading">
				<h3 class="kanban-card__title">
					<RouterLink
						:to="{ name: 'task.detail', params: {id: task.id} }"
						class="kanban-card__title-link"
						draggable="false"
						@click.exact.prevent.stop="openTaskDetail()"
						@click.ctrl.stop
						@click.meta.stop
					>
						{{ task.title }}
					</RouterLink>
				</h3>
				<Done
					class="kanban-card__done"
					:is-done="task.done"
					variant="small"
				/>
			</div>

			<span
				v-if="projectTitle"
				class="project-title"
			>
				{{ projectTitle }}
			</span>

			<ProgressBar
				v-if="task.percent_done > 0"
				class="task-progress"
				:value="task.percent_done * 100"
			/>

			<div class="kanban-card__chips">
				<Labels :labels="task.labels" />
				<PriorityLabel
					:priority="task.priority"
					:done="task.done"
				/>
			</div>

			<div class="footer">
				<span class="task-id">
					{{ getTaskIdentifier(task) }}
					<span
						v-if="showTaskPosition"
						class="tw:text-red-600 tw:ps-2"
					>
						{{ task.position }}
					</span>
				</span>
				<span
					v-if="new Date(task.due_date ?? 0).getTime() > 0"
					v-tooltip="formatDateLong(task.due_date)"
					class="due-date"
				>
					<OverdueIndicator v-if="isOverdue" />
					<Icon
						v-else
						:icon="['far', 'calendar-alt']"
					/>
					<time :datetime="formatISO(task.due_date)">
						{{ formatDisplayDate(task.due_date) }}
					</time>
				</span>
				<span
					v-if="task.attachments.length > 0"
					class="icon"
					role="img"
					:aria-label="$t('task.attributes.attachment', task.attachments.length)"
				>
					<Icon icon="paperclip" />
				</span>
				<span
					v-if="!isEditorContentEmpty(task.description)"
					class="icon"
				>
					<Icon icon="align-left" />
				</span>
				<span
					v-if="task.repeat_after > 0"
					class="icon"
				>
					<Icon icon="history" />
				</span>
				<CommentCount
					:task="task"
					class="project-task-icon"
				/>
				<ChecklistSummary
					:task="task"
					class="checklist"
				/>
				<AssigneeList
					v-if="task.assignees.length > 0"
					class="kanban-card__assignees"
					:assignees="task.assignees"
					:avatar-size="20"
				/>
			</div>
		</div>
	</div>
</template>

<script lang="ts" setup>
import {computed, onBeforeUnmount, onMounted, ref, useTemplateRef, watch} from 'vue'
import {useRouter} from 'vue-router'

import {useGlobalNow} from '@/composables/useGlobalNow'

import PriorityLabel from '@/components/tasks/partials/PriorityLabel.vue'
import OverdueIndicator from '@/components/tasks/partials/OverdueIndicator.vue'
import ProgressBar from '@/components/misc/ProgressBar.vue'
import Done from '@/components/misc/Done.vue'
import Labels from '@/components/tasks/partials/Labels.vue'
import ChecklistSummary from './ChecklistSummary.vue'
import CommentCount from './CommentCount.vue'

import {getHexColor, getTaskIdentifier} from '@/helpers/task'
import type {Task as ITask} from '@/client/generated'
import type {TaskResponse} from '@/client/queries/tasks'
import {canPreviewImage} from '@/helpers/attachmentPreview'
import {fetchAttachmentUrl, releaseAttachmentUrl} from '@/helpers/attachments'
import {ApiError} from '@/client/problem'

import {formatDateLong, formatDisplayDate, formatISO} from '@/helpers/time/formatDate'
import {useUpdateTaskMutation} from '@/client/queries/taskMutations'
import AssigneeList from '@/components/tasks/partials/AssigneeList.vue'
import {playPopSound} from '@/helpers/playPop'
import {isEditorContentEmpty} from '@/helpers/editorContentEmpty'
import {useProjects} from '@/composables/useProjects'
import {useDelayedLoading} from '@/composables/useDelayedLoading'
import {TASK_REPEAT_MODES} from '@/types/IRepeatMode'

const props = defineProps<{
	task: TaskResponse,
	projectId: number,
}>()

const emit = defineEmits<{
	'taskCompletedRecurring': [task: ITask]
}>()

const router = useRouter()
const updateTask = useUpdateTaskMutation()

const loadingInternal = ref(false)
const showLoading = useDelayedLoading(loadingInternal)

const color = computed(() => getHexColor(props.task.hex_color))

const projectList = useProjects()

const projectTitle = computed(() => {
	if (props.projectId === props.task.project_id) {
		return
	}
	
	const project = projectList.projects[props.task.project_id]
	return project?.title
})

const showTaskPosition = computed(() => window.DEBUG_TASK_POSITION)

const {now} = useGlobalNow()
const isOverdue = computed(() => (
	!props.task.done &&
	props.task.due_date !== null &&
	new Date(props.task.due_date ?? 0).getTime() > 0 &&
	new Date(props.task.due_date ?? 0).getTime() <= now.value.getTime()
))

async function toggleTaskDone(task: TaskResponse) {
	const isRecurringTask = task.repeat_after > 0 || task.repeat_mode === TASK_REPEAT_MODES.REPEAT_MODE_MONTH
	const wasBeingMarkedDone = !task.done
	
	loadingInternal.value = true
	try {
		const updatedTask = await updateTask.mutateAsync({
			...task,
			done: !task.done,
		})

		if (updatedTask.done) {
			playPopSound()
		}
		
		// Emit event if this was a recurring task being marked as done
		if (isRecurringTask && wasBeingMarkedDone && updatedTask.done) {
			emit('taskCompletedRecurring', updatedTask)
		}
	} finally {
		loadingInternal.value = false
	}
}

function openTaskDetail() {
	router.push({
		name: 'task.detail',
		params: {id: props.task.id},
		state: {backdropView: router.currentRoute.value.fullPath},
	})
}

const coverImageBlobUrl = ref<string | null>(null)

function showCoverImage(url: string | null) {
	releaseAttachmentUrl(coverImageBlobUrl.value)
	coverImageBlobUrl.value = url
}

let coverRequest = 0
let coverRetry: ReturnType<typeof setTimeout> | undefined
// the server answers 503 while its preview queue is full
const COVER_RETRY_DELAY = 5000

async function maybeDownloadCoverImage(retryWhenBusy = true) {
	clearTimeout(coverRetry)
	const request = ++coverRequest
	if (!props.task.cover_image_attachment_id) {
		showCoverImage(null)
		return
	}

	const attachment = props.task.attachments.find(a => a.id === props.task.cover_image_attachment_id)
	if (!attachment || !canPreviewImage(attachment)) {
		return
	}

	try {
		const url = await fetchAttachmentUrl({id: attachment.id!, task_id: props.task.id}, 'lg')
		if (request !== coverRequest) {
			releaseAttachmentUrl(url)
			return
		}
		showCoverImage(url)
	} catch (error) {
		// the cover is decorative, the card works without it
		if (retryWhenBusy && request === coverRequest && error instanceof ApiError && error.status === 503) {
			coverRetry = setTimeout(() => maybeDownloadCoverImage(false), COVER_RETRY_DELAY)
		}
	}
}

const card = useTemplateRef<HTMLElement>('card')
const hasBeenVisible = ref(false)
let visibilityObserver: IntersectionObserver | undefined

// rootMargin only grows the viewport; buckets scroll on their own and need scrollMargin
const COVER_PRELOAD_MARGIN = '200px'

onMounted(() => {
	visibilityObserver = new IntersectionObserver(
		entries => {
			if (!entries.some(entry => entry.isIntersecting)) {
				return
			}
			hasBeenVisible.value = true
			visibilityObserver?.disconnect()
		},
		{
			rootMargin: COVER_PRELOAD_MARGIN,
			scrollMargin: COVER_PRELOAD_MARGIN,
		},
	)
	visibilityObserver.observe(card.value!)
})

watch(
	[hasBeenVisible, () => props.task.cover_image_attachment_id],
	([visible]) => {
		if (visible) {
			maybeDownloadCoverImage()
		}
	},
)

onBeforeUnmount(() => {
	visibilityObserver?.disconnect()
	clearTimeout(coverRetry)
	coverRequest++
	showCoverImage(null)
})
</script>

<style lang="scss" scoped>
// prime-tasks: Yuyu card (docs/DESIGN.md §4.7 in Yuyu): canvas, 1px border, r-lg, flat until hover.
.task {
	-webkit-touch-callout: none; // iOS Safari
	user-select: none;
	cursor: pointer;
	position: relative;
	display: block;
	overflow: hidden;

	background: var(--yy-bg-canvas, var(--white));
	border: 1px solid var(--yy-border, var(--grey-200));
	border-radius: var(--yy-radius-md, 12px); // compact: r-md
	box-shadow: none;
	color: var(--yy-text-secondary, var(--text));
	font-family: var(--yy-font-body, $family-sans-serif);
	font-size: .875rem;
	transition: box-shadow var(--yy-duration-hover, 180ms) var(--yy-ease, ease);

	&:hover,
	&:focus-within {
		box-shadow: var(--yy-shadow-float, var(--shadow-md));
	}

	&.loader-container.is-loading::after {
		inline-size: 1.5rem;
		block-size: 1.5rem;
		inset-block-start: calc(50% - .75rem);
		inset-inline-start: calc(50% - .75rem);
		border-width: 2px;
	}

	&.is-moving {
		opacity: .5;
	}
}

// The task's own colour: a slim band along the top edge, like Yuyu's accent band.
.kanban-card__band {
	position: absolute;
	inset-block-start: 0;
	inset-inline: 0;
	z-index: 1;
	block-size: 3px;
}

.kanban-card__cover {
	display: block;
	inline-size: 100%;
	max-block-size: 112px;
	object-fit: cover;
	border-block-end: 1px solid var(--yy-border, var(--grey-200));
}

.kanban-card__body {
	display: flex;
	flex-direction: column;
	gap: 6px;
	padding: var(--yy-space-2, .5rem) var(--yy-space-3, .75rem);
}

.has-cover .kanban-card__body,
.kanban-card__band + .kanban-card__body {
	padding-block-start: calc(var(--yy-space-2, .5rem) + 2px);
}

.kanban-card__heading {
	display: flex;
	align-items: flex-start;
	justify-content: space-between;
	gap: var(--yy-space-2, .5rem);
}

.task-id,
.project-title {
	display: flex;
	align-items: center;
	color: var(--yy-text-muted, var(--grey-500));
	font-size: .6875rem;
	font-weight: 600;
	letter-spacing: .02em;
	font-variant-numeric: tabular-nums;
}

.project-title {
	margin-block-start: -2px;
}

// Doubled class to outrank the global :root:root h3 heading rule in theme/yuyu.scss.
h3.kanban-card__title.kanban-card__title {
	margin: 0;
	color: var(--yy-text, var(--text-strong));
	font-family: var(--yy-font-body, $family-sans-serif);
	font-size: .875rem;
	font-weight: 600 !important; // upstream theme.scss and yuyu.scss force heading weights with !important
	letter-spacing: 0;
	line-height: 1.3;
	text-wrap: pretty;
	overflow-wrap: anywhere;
}

.kanban-card__title-link {
	color: inherit;
	text-decoration: none;

	&:hover {
		color: inherit;
	}
}

.task-progress {
	margin: 0;
	inline-size: 100%;
	block-size: .25rem;
}

// Labels and priority as Yuyu chips (pills, 12px, weight 700).
.kanban-card__chips {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 4px;

	&:empty {
		display: none;
	}

	:deep(.label-wrapper) {
		gap: 4px;
	}

	:deep(.tag) {
		block-size: auto;
		padding: 1px 8px;
		font-size: .6875rem;
		font-weight: 700;
		line-height: 1.4;
	}

	:deep(.priority-label) {
		display: inline-flex;
		align-items: center;
		gap: 3px;
		padding: 1px 8px;
		border-radius: var(--yy-radius-full, 999px);
		background: var(--yy-bg-muted, var(--grey-100));
		color: var(--yy-text-secondary, var(--grey-700));
		font-size: .6875rem;
		font-weight: 700;
		line-height: 1.4;

		.icon {
			block-size: auto;
			inline-size: auto;
			padding: 0;
			font-size: .6875rem;
		}
	}

	:deep(.priority-label.high-priority) {
		background: var(--yy-danger-bg, #FEE2E2);
		color: var(--yy-danger-fg, #B91C1C);
	}

	:deep(.priority-label.not-so-high) {
		background: var(--yy-warning-bg, #FEF3C7);
		color: var(--yy-warning-fg, #92400E);
	}
}

// Meta row: identifier, due date, indicators, assignees. Compact: no hairline, one tight line.
.footer {
	display: flex;
	flex-wrap: wrap;
	align-items: center;
	gap: 2px var(--yy-space-2, .5rem);
	margin: 0;
	padding: 0;
	border: 0;
	background: transparent;
	color: var(--yy-text-muted, var(--grey-500));
	font-size: .75rem;

	&:empty {
		display: none;
	}

	.icon {
		inline-size: auto;
		block-size: auto;
		color: var(--yy-text-muted, var(--grey-500));
	}

	:deep(.comment-count) {
		color: var(--yy-text-muted, var(--grey-500));
		font-size: .75rem;
	}

	:deep(.checklist-summary) {
		padding-inline-start: 0;
		color: var(--yy-text-muted, var(--grey-500));
		font-size: .75rem;
	}
}

.due-date {
	display: inline-flex;
	align-items: center;
	gap: var(--yy-space-1, .25rem);
	color: var(--yy-text-secondary, var(--grey-700));
	font-weight: 600;
	white-space: nowrap;

	.task[data-is-overdue] & {
		color: var(--yy-danger-fg, var(--danger-text));
	}
}

.kanban-card__assignees {
	margin-inline-start: auto;

	:deep(.user img) {
		border-color: var(--yy-bg-canvas, var(--white));
	}
}

// Done cards stay visibly done: the Done chip, plus a quieter title and cover.
.kanban-card--done {
	h3.kanban-card__title.kanban-card__title {
		color: var(--yy-text-muted, var(--grey-500));
		text-decoration: line-through;
		text-decoration-thickness: 1px;
	}

	.kanban-card__cover,
	.kanban-card__band {
		opacity: .6;
	}
}

.kanban-card__done {
	flex-shrink: 0;
}
</style>
