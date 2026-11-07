<template>
	<div
		:class="{ 'is-loading': showLoading}"
		class="loader-container"
	>
		<XButton
			:to="{name:'labels.create'}"
			class="is-pulled-end"
			icon="plus"
		>
			{{ $t('label.create.header') }}
		</XButton>

		<div class="content">
			<h1>{{ $t('label.manage') }}</h1>
			<p
				v-if="labels.length > 0"
				class="labels-caption"
			>
				{{ $t('label.description') }}
			</p>
			<p
				v-else
				class="has-text-centered has-text-grey is-italic"
			>
				{{ $t('label.newCTA') }}
				<RouterLink :to="{name:'labels.create'}">
					{{ $t('label.create.title') }}.
				</RouterLink>
			</p>
		</div>

		<div class="columns">
			<!-- prime-tasks: Yuyu label cards (swatch, name, muted description) instead of pills -->
			<ul class="labels-list column">
				<li
					v-for="label in labels"
					:key="label.id"
					class="label-card"
					:class="{'is-editing': isLabelEdit && labelEditLabel.id === label.id}"
				>
					<span
						class="label-card__swatch"
						:class="{'is-neutral': !getLabelColor(label)}"
						:style="{'background-color': getLabelColor(label) || undefined}"
						aria-hidden="true"
					/>
					<div class="label-card__body">
						<RouterLink
							:to="{name: 'home', query: {labels: label.id}}"
							class="label-card__link"
						>
							<span class="label-card__name">{{ label.title }}</span>
						</RouterLink>
						<p
							v-if="labelDescription(label)"
							class="label-card__description"
						>
							{{ labelDescription(label) }}
						</p>
					</div>
					<BaseButton
						v-if="userInfo?.id === label.created_by?.id"
						v-tooltip="$t('label.edit.header')"
						class="label-edit-button"
						:aria-label="$t('label.edit.header')"
						@click.stop.prevent="editLabel(label)"
					>
						<Icon
							icon="pen"
							class="icon"
						/>
					</BaseButton>
				</li>
			</ul>
			<div
				v-if="isLabelEdit"
				class="column is-4"
			>
				<Card
					:title="$t('label.edit.header')"
					:show-close="true"
					@close="() => isLabelEdit = false"
				>
					<form @submit.prevent="editLabelSubmit()">
						<FormField
							v-model="labelEditLabel.title"
							:label="$t('label.attributes.title')"
							:placeholder="$t('label.attributes.titlePlaceholder')"
							type="text"
						/>
						<FormField :label="$t('label.attributes.description')">
							<Editor
								v-if="editorActive"
								v-model="labelEditLabel.description"
								:placeholder="$t('label.attributes.description')"
							/>
						</FormField>
						<FormField :label="$t('label.attributes.color')">
							<ColorPicker v-model="labelEditLabel.hex_color" />
						</FormField>
						<div class="field has-addons">
							<div class="control is-expanded">
								<XButton
									:loading="loading"
									class="is-fullwidth"
									type="submit"
								>
									{{ $t('misc.save') }}
								</XButton>
							</div>
							<div class="control">
								<XButton
									icon="trash-alt"
									danger
									:aria-label="$t('task.label.delete.header')"
									@click="showDeleteDialoge(labelEditLabel)"
								/>
							</div>
						</div>
					</form>
				</Card>
			</div>

			<Modal
				:enabled="showDeleteModal"
				@close="showDeleteModal = false"
				@submit="deleteLabel(labelToDelete)"
			>
				<template #header>
					<span>{{ $t('task.label.delete.header') }}</span>
				</template>

				<template #text>
					<p>
						{{ $t('task.label.delete.text1') }}<br>
						{{ $t('task.label.delete.text2') }}
					</p>
				</template>
			</Modal>
		</div>
	</div>
</template>

<script setup lang="ts">
import {computed, nextTick, ref} from 'vue'
import {useI18n} from 'vue-i18n'

import BaseButton from '@/components/base/BaseButton.vue'
import Editor from '@/components/input/AsyncEditor'
import ColorPicker from '@/components/input/ColorPicker.vue'
import FormField from '@/components/input/FormField.vue'

import {useAuthStore} from '@/stores/auth'

import { useTitle } from '@/composables/useTitle'
import {getLabelColor} from '@/composables/useLabelStyles'
import {htmlToPreviewText} from '@/helpers/htmlToPreviewText'
import {useLabels} from '@/composables/useLabels'
import {useDelayedLoading} from '@/composables/useDelayedLoading'
import {
	type UpdateLabelInput,
	useDeleteLabelMutation,
	useUpdateLabelMutation,
} from '@/client/queries/labels'
import type {Label} from '@/client/generated'

const {t} = useI18n({useScope: 'global'})

const labelEditLabel = ref<Required<UpdateLabelInput>>({id: 0, title: '', description: '', hex_color: ''})
const isLabelEdit = ref(false)
const editorActive = ref(false)
const showDeleteModal = ref(false)
const labelToDelete = ref<Label | undefined>(undefined)

useTitle(() => t('label.title'))

const authStore = useAuthStore()
const userInfo = computed(() => authStore.info)

const {labels, isPending} = useLabels()
const updateLabelMutation = useUpdateLabelMutation()
const deleteLabelMutation = useDeleteLabelMutation()
const loading = computed(() => isPending.value || updateLabelMutation.isPending.value || deleteLabelMutation.isPending.value)
const showLoading = useDelayedLoading(loading)

function labelDescription(label: Label) {
	return htmlToPreviewText(label.description ?? '')
}

function deleteLabel(label?: Label) {
	if (!label) {
		return
	}

	showDeleteModal.value = false
	isLabelEdit.value = false
	return deleteLabelMutation.mutateAsync(label)
}

function editLabelSubmit() {
	return updateLabelMutation.mutateAsync(labelEditLabel.value)
}

function editLabel(label: Label) {
	if (typeof label.id === 'undefined' || label.created_by?.id !== userInfo.value?.id) {
		return
	}
	labelEditLabel.value = {
		id: label.id,
		title: label.title ?? '',
		description: label.description ?? '',
		hex_color: label.hex_color ?? '',
	}
	isLabelEdit.value = true

	// This makes the editor trigger its mounted function again which makes it forget every input
	// it currently has in its textarea. This is a counter-hack to a hack inside of vue-easymde
	// which made it impossible to detect change from the outside. Therefore the component would
	// not update if new content from the outside was made available.
	// See https://github.com/NikulinIlya/vue-easymde/issues/3
	editorActive.value = false
	nextTick(() => editorActive.value = true)
}

function showDeleteDialoge(label: Label) {
	labelToDelete.value = label
	showDeleteModal.value = true
}
</script>

<style lang="scss" scoped>
// prime-tasks: Yuyu label cards (docs/DESIGN.md §4.7 in Yuyu): canvas, 1px border, r-lg, flat until hover.
.labels-caption {
	max-inline-size: 60ch;
	color: var(--yy-text-muted, var(--grey-500));
	font-size: .875rem;
}

.labels-list {
	display: grid;
	grid-template-columns: repeat(auto-fill, minmax(220px, 1fr));
	gap: var(--yy-space-3, .75rem);
	align-content: start;
	margin: 0;
	list-style: none;
}

.label-card {
	position: relative;
	display: flex;
	align-items: flex-start;
	gap: var(--yy-space-3, .75rem);
	padding: var(--yy-space-4, 1rem);
	background: var(--yy-bg-canvas, var(--white));
	border: 1px solid var(--yy-border, var(--grey-200));
	border-radius: var(--yy-radius-lg, 16px);
	transition: background-color var(--yy-duration-hover, 180ms) var(--yy-ease, ease);

	&:hover,
	&:focus-within {
		background: var(--yy-bg-subtle, var(--grey-50));
	}

	&.is-editing {
		border-color: var(--yy-text, var(--text));
	}
}

.label-card__swatch {
	flex-shrink: 0;
	inline-size: 32px;
	block-size: 32px;
	border-radius: var(--yy-radius-sm, 8px);
	// A hairline keeps very light label colours visible on the canvas.
	box-shadow: inset 0 0 0 1px rgb(0 0 0 / 8%);

	&.is-neutral {
		background: var(--yy-bg-emphasis, var(--grey-200));
	}
}

.label-card__body {
	display: flex;
	flex: 1;
	flex-direction: column;
	gap: var(--yy-space-1, .25rem);
	min-inline-size: 0;
	padding-block-start: 4px;
}

// The whole card is the link (stretched), the edit button sits above it.
.label-card__link {
	color: var(--yy-text, var(--text));

	&::after {
		content: '';
		position: absolute;
		inset: 0;
		z-index: 1;
		border-radius: inherit;
	}

	&:focus-visible {
		outline: none;

		&::after {
			outline: 2px solid var(--yy-text, var(--primary));
			outline-offset: 2px;
		}
	}
}

.label-card__name {
	font-family: var(--yy-font-display, $vikunja-font);
	font-weight: 800;
	font-size: 1rem;
	line-height: 1.3;
	overflow-wrap: anywhere;
}

.label-card__description {
	margin: 0;
	color: var(--yy-text-muted, var(--grey-500));
	font-size: .8125rem;
	line-height: 1.45;
	overflow: hidden;
	overflow-wrap: anywhere;

	display: -webkit-box;
	-webkit-line-clamp: 2;
	-webkit-box-orient: vertical;
}

.label-edit-button {
	position: relative;
	z-index: 2;
	display: inline-flex;
	flex-shrink: 0;
	align-items: center;
	justify-content: center;
	inline-size: 32px;
	block-size: 32px;
	border-radius: var(--yy-radius-full, 999px);
	color: var(--yy-text-muted, var(--grey-500));
	transition: background-color var(--yy-duration-hover, 180ms) var(--yy-ease, ease), color var(--yy-duration-hover, 180ms) var(--yy-ease, ease);

	&:hover {
		background: var(--yy-bg-emphasis, var(--grey-200));
		color: var(--yy-text, var(--text));
	}

	.icon {
		inline-size: 14px;
		block-size: 14px;
		font-size: 14px;
	}
}
</style>
