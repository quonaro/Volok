<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { useQuery, useMutation, useQueryClient } from '@tanstack/vue-query'
import type { Group, UpdateGroup } from '~/utils/schemas/group'
import { updateGroup } from '~/utils/services/group'
import { fetchInbounds } from '~/utils/services/inbound'
import UiButton from '~/components/ui/button/button.vue'
import UiInput from '~/components/ui/input/input.vue'
import UiLabel from '~/components/ui/label/label.vue'
import UiSelect from '~/components/ui/select/select.vue'
import {
  Sheet,
  SheetContent,
  SheetHeader,
  SheetFooter,
  SheetTitle,
  SheetDescription,
} from '~/components/ui/sheet'

const props = defineProps<{
  group: Group | null
  open: boolean
}>()

const emit = defineEmits<{
  (e: 'update:open', value: boolean): void
}>()

const queryClient = useQueryClient()

const { data: inbounds } = useQuery({
  queryKey: ['inbounds'],
  queryFn: () => fetchInbounds(),
})

const inboundOptions = computed(() => [
  { label: 'None', value: '' },
  ...(inbounds.value ?? []).map((ib) => ({ label: ib.type, value: ib.id })),
])

const name = ref('')
const inboundId = ref('')
const randomEnabled = ref(false)
const randomLimit = ref<number | undefined>(undefined)
const showOrigins = ref(false)

const selectedInbound = computed(() => inbounds.value?.find((ib) => ib.id === inboundId.value))
const canShowOrigins = computed(() => selectedInbound.value?.type === 'vless')

watch(canShowOrigins, (allowed) => {
  if (!allowed) showOrigins.value = false
})

const updateMutation = useMutation({
  mutationFn: ({ id, data }: { id: string; data: UpdateGroup }) => updateGroup(id, data),
  onSuccess: () => {
    queryClient.invalidateQueries({ queryKey: ['groups'] })
    close()
  },
})

const isSubmitting = computed(() => updateMutation.isPending.value)

watch(
  () => props.open,
  (open) => {
    if (!open || !props.group) return
    name.value = props.group.name
    inboundId.value = props.group.inbound_id ?? ''
    randomEnabled.value = props.group.random_enabled ?? false
    randomLimit.value = props.group.random_limit ?? undefined
    showOrigins.value = props.group.show_origins ?? false
  }
)

function buildPayload(): UpdateGroup {
  return {
    name: name.value,
    inbound_id: inboundId.value,
    random_enabled: randomEnabled.value,
    random_limit: randomLimit.value ?? null,
    show_origins: canShowOrigins.value ? showOrigins.value : false,
  }
}

function save() {
  if (!props.group || !name.value.trim()) return
  updateMutation.mutate({ id: props.group.id, data: buildPayload() })
}

function close() {
  emit('update:open', false)
}
</script>

<template>
  <Sheet :open="props.open" @update:open="emit('update:open', $event)">
    <SheetContent>
      <SheetHeader>
        <SheetTitle>Edit Group</SheetTitle>
        <SheetDescription>Update group settings.</SheetDescription>
      </SheetHeader>
      <div class="grow min-h-0 space-y-4 overflow-y-auto py-4">
        <div class="space-y-2">
          <UiLabel>Group Name</UiLabel>
          <UiInput v-model="name" placeholder="Enter group name" />
        </div>

        <div class="space-y-2">
          <UiLabel>Inbound</UiLabel>
          <UiSelect v-model="inboundId" :options="inboundOptions" />
        </div>

        <div class="flex items-center gap-2">
          <input id="edit-random-enabled" v-model="randomEnabled" type="checkbox" class="h-4 w-4" />
          <UiLabel for="edit-random-enabled">Random selection for subscriptions</UiLabel>
        </div>

        <div class="space-y-2">
          <UiLabel>Limit (optional)</UiLabel>
          <UiInput v-model="randomLimit" type="number" min="1" placeholder="Max nodes to return" />
          <p class="text-xs text-muted-foreground">
            Maximum number of nodes to return in subscriptions
          </p>
        </div>

        <div class="flex items-center gap-2">
          <input
            id="edit-show-origins"
            v-model="showOrigins"
            type="checkbox"
            class="h-4 w-4"
            :disabled="!canShowOrigins"
          />
          <UiLabel for="edit-show-origins" :class="{ 'text-muted-foreground': !canShowOrigins }">
            Show direct node links
            <span v-if="!canShowOrigins" class="text-xs">(VLESS only)</span>
          </UiLabel>
        </div>
      </div>
      <SheetFooter>
        <UiButton variant="outline" @click="close">Cancel</UiButton>
        <UiButton :disabled="!name.trim() || isSubmitting" @click="save">
          {{ isSubmitting ? 'Updating...' : 'Update' }}
        </UiButton>
      </SheetFooter>
    </SheetContent>
  </Sheet>
</template>
