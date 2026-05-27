<template>
  <section class="type-section retouch-form">
    <div class="form-card upload-card">
      <label class="field-label">任务级参考图 / 附件 <span class="required">*</span></label>
      <p class="field-hint">
        上传内容作用于整个 P 图任务，本阶段不支持为每条需求单独绑定参考图或素材文件。
      </p>
      <ReferenceUploadPanel v-model="referenceRefsModel" compact />
    </div>

    <div class="form-card">
      <BaseTextarea
        v-model="localForm.designRequirement"
        label="任务总述（可选）"
        :rows="3"
        placeholder="可填写对整个任务的总体说明；若不填，将使用首条需求描述作为任务总述"
      />
    </div>

    <section class="requirements-section">
      <div class="requirements-header">
        <div>
          <h4 class="requirements-title">P 图需求明细</h4>
          <p class="field-hint">至少填写 1 条需求描述；SKU / 规格 / 备注为可选项。</p>
        </div>
        <button type="button" class="add-req-btn" @click="addRequirement">添加需求</button>
      </div>

      <article
        v-for="(item, index) in retouchRequirements"
        :key="index"
        class="requirement-card"
      >
        <div class="requirement-card-head">
          <span class="requirement-index">需求 {{ index + 1 }}</span>
          <button
            v-if="retouchRequirements.length > 1"
            type="button"
            class="remove-req-btn"
            @click="removeRequirement(index)"
          >
            删除
          </button>
        </div>
        <BaseTextarea
          v-model="item.description"
          label="需求描述"
          :rows="3"
          placeholder="请描述本条 P 图修改要求"
        />
        <div class="optional-grid">
          <BaseInput v-model="item.skuCode" label="SKU / 款号（可选）" placeholder="例如 SKU-001" />
          <BaseInput v-model="item.spec" label="规格（可选）" placeholder="例如 60×40cm" />
        </div>
        <BaseInput v-model="item.remark" label="备注（可选）" placeholder="补充说明" />
      </article>
    </section>
  </section>
</template>

<script setup lang="ts">
import { computed, watch } from 'vue'
import type { TaskCreateFormModel } from '@/domain/types'
import { createEmptyRetouchRequirementDraft } from '@/domain/types/retouch-requirement'
import BaseInput from '@/components/base/BaseInput.vue'
import BaseTextarea from '@/components/base/BaseTextarea.vue'
import ReferenceUploadPanel from '@/components/task/ReferenceUploadPanel.vue'

const props = defineProps<{
  form: TaskCreateFormModel
}>()

const emit = defineEmits<{
  'update:form': [value: TaskCreateFormModel]
}>()

const localForm = computed({
  get: () => props.form,
  set: (value: TaskCreateFormModel) => emit('update:form', value),
})

const referenceRefsModel = computed({
  get: () => localForm.value.referenceFileRefs,
  set: (value: (string | Record<string, unknown>)[]) => {
    localForm.value = { ...localForm.value, referenceFileRefs: value }
  },
})

const retouchRequirements = computed(() => localForm.value.retouchRequirements ?? [])

watch(
  () => localForm.value.retouchRequirements,
  (items) => {
    if (!Array.isArray(items) || items.length === 0) {
      localForm.value = {
        ...localForm.value,
        retouchRequirements: [createEmptyRetouchRequirementDraft(1)],
      }
    }
  },
  { immediate: true },
)

function addRequirement() {
  const next = [...(localForm.value.retouchRequirements ?? [])]
  next.push(createEmptyRetouchRequirementDraft(next.length + 1))
  localForm.value = { ...localForm.value, retouchRequirements: next }
}

function removeRequirement(index: number) {
  const next = [...(localForm.value.retouchRequirements ?? [])]
  if (next.length <= 1) return
  next.splice(index, 1)
  localForm.value = {
    ...localForm.value,
    retouchRequirements: next.map((item, i) => ({ ...item, sortOrder: i + 1 })),
  }
}
</script>

<style scoped>
.retouch-form {
  display: flex;
  flex-direction: column;
  gap: 12px;
}

.field-hint {
  margin: 0 0 8px;
  font-size: 12px;
  line-height: 1.5;
  color: var(--text-secondary, #64748b);
}

.requirements-section {
  display: flex;
  flex-direction: column;
  gap: 10px;
}

.requirements-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
}

.requirements-title {
  margin: 0;
  font-size: 14px;
  font-weight: 600;
}

.add-req-btn,
.remove-req-btn {
  border: 1px solid var(--border-color, #dbe3ef);
  background: #fff;
  color: var(--text-primary, #0f172a);
  border-radius: 6px;
  padding: 4px 10px;
  font-size: 12px;
  cursor: pointer;
}

.requirement-card {
  display: flex;
  flex-direction: column;
  gap: 10px;
  padding: 12px;
  border: 1px solid var(--border-color, #e2e8f0);
  border-radius: 10px;
  background: #f8fafc;
}

.requirement-card-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.requirement-index {
  font-size: 13px;
  font-weight: 600;
  color: var(--text-primary, #0f172a);
}

.optional-grid {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 10px;
}

@media (max-width: 720px) {
  .optional-grid {
    grid-template-columns: 1fr;
  }
}
</style>
