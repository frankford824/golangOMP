<script setup lang="ts">
import { computed, ref, shallowRef, watch } from 'vue'
import { parseStringToDateValue, type DateValue } from 'reka-ui/date'
import { CalendarDays, ChevronLeft, ChevronRight } from 'lucide-vue-next'
import {
  CalendarRoot, CalendarHeader, CalendarHeading, CalendarPrev, CalendarNext,
  CalendarGrid, CalendarGridHead, CalendarGridRow, CalendarHeadCell, CalendarGridBody,
  CalendarCell, CalendarCellTrigger, PopoverRoot, PopoverTrigger, PopoverPortal, PopoverContent,
} from 'reka-ui'
import { formatShanghaiDateTime } from '@aw/shared/format/dateTime'

const value = defineModel<string>({ required: true })
const props = defineProps<{
  label: string
  min?: string
  max?: string
  filterKey: string
  loadDays: (month: string, signal: AbortSignal) => Promise<string[]>
}>()

const open = ref(false)
const visibleDate = shallowRef<DateValue>()
const selected = computed(() => parseDate(value.value))
const minDate = computed(() => parseDate(props.min))
const maxDate = computed(() => parseDate(props.max))
const visibleMonth = computed(() => visibleDate.value?.toString().slice(0, 7))
const uploadDays = shallowRef(new Set<string>())
const status = ref<'loading' | 'ready' | 'error'>('loading')
const hasUploads = computed(() => status.value === 'ready' && uploadDays.value.size > 0)
const calendarStatus = computed(() => {
  if (status.value === 'loading') return '正在读取上传日期…'
  if (status.value === 'error') return '上传日期标记加载失败'
  if (hasUploads.value) return `本月 ${uploadDays.value.size} 天有上传`
  return '本月没有匹配的上传'
})

function parseDate(date?: string) {
  return date && visibleDate.value ? parseStringToDateValue(date, visibleDate.value) : undefined
}

function anchorCalendar(reference: DateValue) {
  const date = value.value || props.min || props.max || formatShanghaiDateTime(new Date()).slice(0, 10)
  visibleDate.value = parseStringToDateValue(date, reference)
}

function setOpen(next: boolean) {
  if (next && visibleDate.value) anchorCalendar(visibleDate.value)
  open.value = next
}

function selectDate(date: DateValue | DateValue[] | undefined) {
  if (Array.isArray(date)) return
  value.value = date?.toString() ?? ''
  open.value = false
}

watch([open, visibleMonth, () => props.filterKey], ([isOpen, month], _, onCleanup) => {
  uploadDays.value = new Set()
  status.value = 'loading'
  if (!isOpen || !month) return

  const controller = new AbortController()
  const timer = setTimeout(async () => {
    try {
      const dates = await props.loadDays(month, controller.signal)
      if (controller.signal.aborted) return
      uploadDays.value = new Set(dates)
      status.value = 'ready'
    } catch {
      if (!controller.signal.aborted) status.value = 'error'
    }
  }, 160)
  onCleanup(() => {
    clearTimeout(timer)
    controller.abort()
  })
})
</script>

<template>
  <div class="aw-upload-date-field">
    <span class="aw-upload-date-field__label">{{ label }}</span>
    <PopoverRoot :open="open" @update:open="setOpen">
      <PopoverTrigger class="aw-upload-date-field__trigger" type="button" :aria-label="`${label}${value ? `，${value}` : ''}`">
        <span>{{ value || '选择日期' }}</span>
        <CalendarDays :size="15" aria-hidden="true" />
      </PopoverTrigger>
      <PopoverPortal>
        <PopoverContent class="aw-token-scope aw-upload-calendar" align="start" :side-offset="6" :collision-padding="8" :aria-label="label">
          <CalendarRoot
            v-slot="{ date, grid, weekDays }"
            v-model:placeholder="visibleDate"
            :model-value="selected"
            :min-value="minDate"
            :max-value="maxDate"
            :calendar-label="label"
            :week-starts-on="1"
            locale="zh-CN"
            prevent-deselect
            initial-focus
            @update:model-value="selectDate"
          >
            <CalendarHeader :ref="element => { if (element && !visibleDate) anchorCalendar(date) }" class="aw-upload-calendar__header">
              <CalendarHeading />
              <div>
                <CalendarPrev type="button" aria-label="上个月"><ChevronLeft :size="16" /></CalendarPrev>
                <CalendarNext type="button" aria-label="下个月"><ChevronRight :size="16" /></CalendarNext>
              </div>
            </CalendarHeader>
            <CalendarGrid v-for="month in grid" :key="month.value.toString()">
              <CalendarGridHead>
                <CalendarGridRow>
                  <CalendarHeadCell v-for="weekday in weekDays" :key="weekday">{{ weekday }}</CalendarHeadCell>
                </CalendarGridRow>
              </CalendarGridHead>
              <CalendarGridBody>
                <CalendarGridRow v-for="(week, index) in month.rows" :key="index">
                  <CalendarCell v-for="day in week" :key="day.toString()" :date="day">
                    <CalendarCellTrigger
                      :day="day"
                      :month="month.value"
                      :data-uploaded="uploadDays.has(day.toString()) || undefined"
                      :aria-description="uploadDays.has(day.toString()) ? '有上传' : undefined"
                    />
                  </CalendarCell>
                </CalendarGridRow>
              </CalendarGridBody>
            </CalendarGrid>
          </CalendarRoot>
          <div class="aw-upload-calendar__footer">
            <span class="aw-upload-calendar__status" role="status">
              <span v-if="hasUploads" class="aw-upload-calendar__legend" aria-hidden="true" />
              {{ calendarStatus }}
            </span>
            <button v-if="value" class="aw-grid-button" type="button" @click="selectDate(undefined)">清除日期</button>
          </div>
        </PopoverContent>
      </PopoverPortal>
    </PopoverRoot>
  </div>
</template>
