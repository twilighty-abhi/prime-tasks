<template>
	<div
		ref="root"
		class="project-filter-switch"
		role="radiogroup"
		:aria-label="$t('navigation.projectFilter.label')"
	>
		<button
			v-for="filter in SIDEBAR_PROJECT_FILTERS"
			:key="filter"
			type="button"
			role="radio"
			class="project-filter-switch__option"
			:class="{'is-active': modelValue === filter}"
			:aria-checked="modelValue === filter"
			:tabindex="modelValue === filter ? 0 : -1"
			@click="emit('update:modelValue', filter)"
			@keydown.left.prevent="move(-1)"
			@keydown.right.prevent="move(1)"
		>
			{{ $t(`navigation.projectFilter.${filter}`) }}
		</button>
	</div>
</template>

<script setup lang="ts">
// prime-tasks F3: All / Personal / Shared segmented control above the sidebar's project lists.
import {nextTick, useTemplateRef} from 'vue'

import {SIDEBAR_PROJECT_FILTERS, type SidebarProjectFilter} from '@/helpers/projectSharing'

const props = defineProps<{
	modelValue: SidebarProjectFilter
}>()

const emit = defineEmits<{
	'update:modelValue': [filter: SidebarProjectFilter]
}>()

const root = useTemplateRef<HTMLElement>('root')

// Arrow keys move between options, as in any radio group.
async function move(step: number) {
	const index = SIDEBAR_PROJECT_FILTERS.indexOf(props.modelValue)
	const next = SIDEBAR_PROJECT_FILTERS[(index + step + SIDEBAR_PROJECT_FILTERS.length) % SIDEBAR_PROJECT_FILTERS.length]!
	emit('update:modelValue', next)
	await nextTick()
	root.value?.querySelector<HTMLElement>('[aria-checked="true"]')?.focus()
}
</script>

<style lang="scss" scoped>
.project-filter-switch {
	display: flex;
	gap: var(--yy-space-1);
	margin: 0 var(--yy-space-2) var(--yy-space-3);
	padding: var(--yy-space-1);
	background: var(--yy-bg-muted);
	border-radius: var(--yy-radius-full);
}

.project-filter-switch__option {
	flex: 1;
	padding: var(--yy-space-1) var(--yy-space-2);
	border: 0;
	border-radius: var(--yy-radius-full);
	background: transparent;
	color: var(--yy-text-secondary);
	font-family: var(--yy-font-body);
	font-size: 13px;
	font-weight: 600;
	cursor: pointer;
	transition: background-color var(--yy-duration-hover) var(--yy-ease), color var(--yy-duration-hover) var(--yy-ease);

	&:hover {
		background: var(--yy-bg-emphasis);
		color: var(--yy-text);
	}

	&.is-active {
		background: var(--yy-primary);
		color: var(--yy-on-primary);
	}
}
</style>
