<template>
  <div class="legend">
    <span><i class="dot" style="background:#ff4d6d" />高危</span>
    <span><i class="dot" style="background:#ff8a4c" />中高危</span>
    <span><i class="dot" style="background:#ffc857" />中危</span>
    <span><i class="dot" style="background:#4fc3ff" />低危/信息</span>
    <span><i class="ring" />CDN 边缘</span>
    <span class="muted">拖拽/滚轮缩放地图</span>
  </div>
</template>

<script setup lang="ts"></script>

<style scoped>
.legend { display: flex; gap: 14px; align-items: center; font-size: 11px; color: #9db9e8; flex-wrap: wrap; }
.legend span { display: inline-flex; align-items: center; gap: 5px; }
.dot { width: 7px; height: 7px; border-radius: 50%; display: inline-block; }
.ring { width: 7px; height: 7px; border-radius: 50%; border: 1px solid #5ad1ff; display: inline-block; }
.muted { color: #5f7aa8; }
</style>
