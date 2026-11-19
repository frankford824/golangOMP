<script setup lang="ts">
import {ref} from 'vue'
import FeeAccountPicker from '../components/FeeAccountPicker.vue'
import PageHead from '../components/PageHead.vue'
import OAPageReturn from '../components/OAPageReturn.vue'
const type=ref('报销')
</script>
<template><section class="oa-page"><OAPageReturn to="/fees/mine">返回我的申请</OAPageReturn><PageHead title="常用收款账户" /><section class="oa-card"><nav class="oa-actions" aria-label="账户范围"><button type="button" class="oa-button" :aria-pressed="type==='报销'" @click="type='报销'">本人默认账户</button><button type="button" class="oa-button" :aria-pressed="type==='付款'" @click="type='付款'">供应商账户</button></nav><FeeAccountPicker :type="type" manage /></section></section></template>
