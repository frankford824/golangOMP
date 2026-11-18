<script setup lang="ts">
import { computed, ref, useId } from 'vue'
import { ChevronDown } from 'lucide-vue-next'
import { PopoverContent, PopoverPortal, PopoverRoot, PopoverTrigger } from 'reka-ui'

const props = defineProps<{ options: { value: number | 'unassigned'; label: string }[] }>()
const selected = defineModel<(number | 'unassigned')[]>({ required: true })
const open = ref(false)
const panelId = useId()
const selectedLabels = computed(() =>
  props.options.filter(option => selected.value.includes(option.value)).map(option => option.label),
)

function toggle(value: number | 'unassigned', checked: boolean) {
  selected.value = checked ? [...selected.value, value] : selected.value.filter(item => item !== value)
}
</script>

<template>
  <div class="aw-ledger-category">
    <span class="aw-ledger-category__label">分类</span>
    <PopoverRoot v-model:open="open">
      <PopoverTrigger as-child>
        <button class="aw-ledger-category__trigger" type="button" aria-label="分类筛选"
          :aria-controls="open ? panelId : undefined"
          :title="selectedLabels.join('、') || '全部分类'">
          <span>{{ selectedLabels.join('、') || '全部分类' }}</span>
          <ChevronDown :size="15" aria-hidden="true" />
        </button>
      </PopoverTrigger>
      <PopoverPortal>
        <PopoverContent as-child align="start" :side-offset="6" :collision-padding="8">
          <div :id="panelId" class="aw-token-scope aw-ledger-category__panel">
            <button class="aw-ledger-category__option" type="button"
              :aria-pressed="selected.length === 0" @click="selected = []">
              全部分类
            </button>
            <div class="aw-ledger-category__options" role="group" aria-label="分类选项">
              <label v-for="option in options" :key="option.value" class="aw-ledger-category__option">
                <input type="checkbox" :checked="selected.includes(option.value)"
                  @change="toggle(option.value, ($event.target as HTMLInputElement).checked)" />
                <span>{{ option.label }}</span>
              </label>
            </div>
            <div class="aw-ledger-category__footer">
              <span>{{ selected.length ? '已选 ' + selected.length + ' 项' : '全部分类' }}</span>
              <button class="aw-secondary-button" type="button" @click="open = false">完成</button>
            </div>
          </div>
        </PopoverContent>
      </PopoverPortal>
    </PopoverRoot>
  </div>
</template>
