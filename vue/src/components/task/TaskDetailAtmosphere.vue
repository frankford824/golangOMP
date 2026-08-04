<template>
  <div class="task-atmosphere" aria-hidden="true">
    <canvas ref="canvas" />
    <div class="fallback-glow" />
  </div>
</template>

<script setup lang="ts">
import { onBeforeUnmount, onMounted, ref } from 'vue'

const canvas = ref<HTMLCanvasElement | null>(null)
let frame = 0
let resizeObserver: ResizeObserver | null = null
let gl: WebGL2RenderingContext | null = null

const vertexSource = `#version 300 es
in vec2 a_position;
void main() { gl_Position = vec4(a_position, 0.0, 1.0); }
`

const fragmentSource = `#version 300 es
precision highp float;
uniform vec2 u_resolution;
uniform float u_time;
out vec4 outColor;

float glow(vec2 point, vec2 center, float radius) {
  return smoothstep(radius, 0.0, distance(point, center));
}

void main() {
  vec2 uv = gl_FragCoord.xy / max(u_resolution, vec2(1.0));
  float time = u_time * 0.08;
  vec2 a = vec2(0.18 + sin(time) * 0.05, 0.70 + cos(time * 0.7) * 0.08);
  vec2 b = vec2(0.72 + cos(time * 0.8) * 0.08, 0.38 + sin(time * 0.6) * 0.07);
  float light = glow(uv, a, 0.78) * 0.42 + glow(uv, b, 0.74) * 0.30;
  float veil = smoothstep(0.05, 0.62, light);
  vec3 base = vec3(0.130, 0.220, 0.320);
  vec3 lift = vec3(0.160, 0.300, 0.400);
  vec3 color = mix(base, lift, veil * 0.38);
  outColor = vec4(color, 0.94);
}
`

function compileShader(context: WebGL2RenderingContext, type: number, source: string) {
  const shader = context.createShader(type)
  if (!shader) return null
  context.shaderSource(shader, source)
  context.compileShader(shader)
  if (!context.getShaderParameter(shader, context.COMPILE_STATUS)) {
    context.deleteShader(shader)
    return null
  }
  return shader
}

function setup() {
  const target = canvas.value
  if (!target || window.matchMedia('(prefers-reduced-motion: reduce)').matches) return
  gl = target.getContext('webgl2', { alpha: true, antialias: false, powerPreference: 'low-power' })
  if (!gl) return
  const vertex = compileShader(gl, gl.VERTEX_SHADER, vertexSource)
  const fragment = compileShader(gl, gl.FRAGMENT_SHADER, fragmentSource)
  if (!vertex || !fragment) return
  const program = gl.createProgram()
  if (!program) return
  gl.attachShader(program, vertex)
  gl.attachShader(program, fragment)
  gl.linkProgram(program)
  if (!gl.getProgramParameter(program, gl.LINK_STATUS)) return
  const buffer = gl.createBuffer()
  gl.bindBuffer(gl.ARRAY_BUFFER, buffer)
  gl.bufferData(gl.ARRAY_BUFFER, new Float32Array([-1, -1, 3, -1, -1, 3]), gl.STATIC_DRAW)
  const position = gl.getAttribLocation(program, 'a_position')
  gl.enableVertexAttribArray(position)
  gl.vertexAttribPointer(position, 2, gl.FLOAT, false, 0, 0)
  const resolution = gl.getUniformLocation(program, 'u_resolution')
  const time = gl.getUniformLocation(program, 'u_time')
  gl.useProgram(program)

  const resize = () => {
    const ratio = Math.min(window.devicePixelRatio || 1, 1.5)
    target.width = Math.max(1, Math.floor(target.clientWidth * ratio))
    target.height = Math.max(1, Math.floor(target.clientHeight * ratio))
    gl?.viewport(0, 0, target.width, target.height)
  }
  resizeObserver = new ResizeObserver(resize)
  resizeObserver.observe(target)
  resize()
  const render = (stamp: number) => {
    if (!gl) return
    gl.uniform2f(resolution, target.width, target.height)
    gl.uniform1f(time, stamp / 1000)
    gl.drawArrays(gl.TRIANGLES, 0, 3)
    frame = requestAnimationFrame(render)
  }
  frame = requestAnimationFrame(render)
}

onMounted(setup)
onBeforeUnmount(() => {
  cancelAnimationFrame(frame)
  resizeObserver?.disconnect()
  gl?.getExtension('WEBGL_lose_context')?.loseContext()
  gl = null
})
</script>

<style scoped>
.task-atmosphere,.task-atmosphere canvas,.fallback-glow{position:absolute;inset:0}.task-atmosphere{overflow:hidden;pointer-events:none}.task-atmosphere canvas{display:block;width:100%;height:100%;min-height:100%;block-size:100%;min-block-size:100%;opacity:.9}.fallback-glow{background:radial-gradient(circle at 18% 52%,rgb(var(--yb-brand-bright)/.07),transparent 62%),radial-gradient(circle at 82% 28%,rgb(var(--yb-brand-accent)/.045),transparent 58%)}
@media(prefers-reduced-motion:reduce){.task-atmosphere canvas{display:none}}
</style>
