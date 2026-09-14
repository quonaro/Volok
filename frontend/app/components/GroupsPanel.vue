<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import { Plus, Pencil, Trash2, ArrowLeft } from 'lucide-vue-next'
import type { Group, CreateGroup, UpdateGroup } from '~/utils/schemas/group'
import { fetchGroups, createGroup, updateGroup, deleteGroup } from '~/utils/services/group'
import { fetchInbounds } from '~/utils/services/inbound'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiLabel from '~/components/ui/label/label.vue'
import UiSelect from '~/components/ui/select/select.vue'

const queryClient = useQueryClient()
const { confirm } = useConfirm()

const { data: groups, isLoading } = useQuery({
  queryKey: ['groups'],
  queryFn: () => fetchGroups(),
})

const { data: inbounds } = useQuery({
  queryKey: ['inbounds'],
  queryFn: () => fetchInbounds(),
})

const inboundOptions = computed(() => [
  { label: 'None', value: '' },
  ...(inbounds.value ?? []).map((ib) => ({ label: ib.type, value: ib.id })),
])

type Mode = 'list' | 'create' | 'edit'
const mode = ref<Mode>('list')
const editingGroup = ref<Group | null>(null)
const formName = ref('')
const formInboundId = ref('')
const formRandomEnabled = ref(false)
const formRandomLimit = ref<number | null>(null)
const formShowOrigins = ref(false)
const isSubmitting = ref(false)
const deletingIds = ref<Set<string>>(new Set())

const createMutation = useMutation({
  mutationFn: (data: CreateGroup) => createGroup(data),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    backToList()
  },
})

const updateMutation = useMutation({
  mutationFn: ({ id, data }: { id: string; data: UpdateGroup }) => updateGroup(id, data),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    backToList()
  },
})

const deleteMutation = useMutation({
  mutationFn: (id: string) => deleteGroup(id),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    queryClient.invalidateQueries({ queryKey: ['nodes'] })
  },
})

function buildPayload(): CreateGroup {
  return {
    name: formName.value,
    inbound_id: formInboundId.value,
    random_enabled: formRandomEnabled.value,
    random_limit: formRandomLimit.value,
    show_origins: canShowOrigins.value ? formShowOrigins.value : false,
  }
}

function showCreateForm() {
  formName.value = ''
  formInboundId.value = ''
  formRandomEnabled.value = false
  formRandomLimit.value = null
  formShowOrigins.value = false
  mode.value = 'create'
}

function showEditForm(group: Group) {
  editingGroup.value = group
  formName.value = group.name
  formInboundId.value = group.inbound_id ?? ''
  formRandomEnabled.value = group.random_enabled ?? false
  formRandomLimit.value = group.random_limit ?? null
  formShowOrigins.value = group.show_origins ?? false
  mode.value = 'edit'
}

function backToList() {
  mode.value = 'list'
  editingGroup.value = null
  isSubmitting.value = false
}

function submitForm() {
  if (!formName.value.trim() || isSubmitting.value) return
  isSubmitting.value = true
  if (mode.value === 'create') {
    createMutation.mutate(buildPayload(), {
      onSettled: () => {
        isSubmitting.value = false
      },
    })
  } else if (mode.value === 'edit' && editingGroup.value) {
    updateMutation.mutate(
      { id: editingGroup.value.id, data: buildPayload() },
      {
        onSettled: () => {
          isSubmitting.value = false
        },
      }
    )
  }
}

async function handleDelete(group: Group) {
  const ok = await confirm({
    title: 'Delete group',
    message: `Delete group "${group.name}"?`,
    variant: 'destructive',
  })
  if (!ok) return
  const next = new Set(deletingIds.value)
  next.add(group.id)
  deletingIds.value = next
  deleteMutation.mutate(group.id, {
    onSettled: () => {
      const current = new Set(deletingIds.value)
      current.delete(group.id)
      deletingIds.value = current
    },
  })
}

function inboundName(id: string): string {
  if (!id) return '—'
  return inbounds.value?.find((ib) => ib.id === id)?.type ?? id
}

const selectedInbound = computed(() => inbounds.value?.find((ib) => ib.id === formInboundId.value))
const canShowOrigins = computed(() => selectedInbound.value?.type === 'vless')

watch(canShowOrigins, (allowed) => {
  if (!allowed) formShowOrigins.value = false
})
</script>

<template>
  <div class="flex h-full flex-col">
    <!-- List mode -->
    <template v-if="mode === 'list'">
      <div class="flex items-center justify-between px-1 pb-3">
        <span class="text-sm font-medium text-muted-foreground">
          {{ groups?.length ?? 0 }} groups
        </span>
        <UiButton size="sm" @click="showCreateForm">
          <Plus class="mr-1 h-4 w-4" />
          New
        </UiButton>
      </div>
      <div class="grow space-y-2 overflow-y-auto">
        <div v-if="isLoading" class="py-8 text-center text-sm text-muted-foreground">
          Loading...
        </div>
        <div
          v-else-if="!groups || groups.length === 0"
          class="py-8 text-center text-sm text-muted-foreground"
        >
          No groups yet
        </div>
        <div
          v-for="group in groups"
          :key="group.id"
          class="group flex items-center justify-between gap-2 rounded-md border bg-card p-3"
        >
          <div class="min-w-0 flex-1">
            <p class="truncate text-sm font-medium">{{ group.name }}</p>
            <p class="truncate text-xs text-muted-foreground">
              {{ group.total_nodes }} nodes · inbound: {{ inboundName(group.inbound_id) }}
            </p>
          </div>
          <div
            class="flex shrink-0 items-center gap-1 opacity-0 transition-opacity group-hover:opacity-100"
          >
            <UiButton variant="ghost" size="icon" class="h-7 w-7" @click="showEditForm(group)">
              <Pencil class="h-3.5 w-3.5" />
            </UiButton>
            <UiButton
              variant="ghost"
              size="icon"
              class="h-7 w-7 text-destructive hover:text-destructive"
              :disabled="deletingIds.has(group.id)"
              @click="handleDelete(group)"
            >
              <Trash2 class="h-3.5 w-3.5" />
            </UiButton>
          </div>
        </div>
      </div>
    </template>

    <!-- Form mode (create / edit) -->
    <template v-else>
      <div class="flex items-center gap-2 px-1 pb-3">
        <UiButton variant="ghost" size="icon" class="h-7 w-7 shrink-0" @click="backToList">
          <ArrowLeft class="h-4 w-4" />
        </UiButton>
        <span class="text-sm font-medium">
          {{ mode === 'create' ? 'Create Group' : 'Edit Group' }}
        </span>
      </div>
      <div class="grow space-y-4 overflow-y-auto">
        <div class="space-y-2">
          <UiLabel>Name</UiLabel>
          <UiInput v-model="formName" placeholder="Group name" />
        </div>
        <div class="space-y-2">
          <UiLabel>Inbound</UiLabel>
          <UiSelect v-model="formInboundId" :options="inboundOptions" />
        </div>
        <div class="flex items-center gap-2">
          <input
            id="gp-random-enabled"
            v-model="formRandomEnabled"
            type="checkbox"
            class="h-4 w-4 rounded border-input"
          />
          <label for="gp-random-enabled" class="text-sm">Random selection</label>
        </div>
        <div class="space-y-2">
          <UiLabel>Limit (optional)</UiLabel>
          <UiInput
            :model-value="formRandomLimit ?? undefined"
            type="number"
            min="1"
            placeholder="Max nodes to return"
            @update:model-value="formRandomLimit = $event === '' ? null : Number($event)"
          />
        </div>
        <div class="flex items-center gap-2">
          <input
            id="gp-show-origins"
            v-model="formShowOrigins"
            type="checkbox"
            class="h-4 w-4 rounded border-input"
            :disabled="!canShowOrigins"
          />
          <label
            for="gp-show-origins"
            class="text-sm"
            :class="{ 'text-muted-foreground': !canShowOrigins }"
          >
            Show direct node links
            <span v-if="!canShowOrigins" class="text-xs">(VLESS only)</span>
          </label>
        </div>
      </div>
      <div class="flex justify-end gap-2 pt-3">
        <UiButton variant="outline" size="sm" @click="backToList">Cancel</UiButton>
        <UiButton size="sm" :disabled="!formName.trim() || isSubmitting" @click="submitForm">
          {{ isSubmitting ? 'Saving...' : mode === 'create' ? 'Create' : 'Save' }}
        </UiButton>
      </div>
    </template>
  </div>
</template>
