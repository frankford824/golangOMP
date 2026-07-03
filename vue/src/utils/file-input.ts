export function snapshotAndResetFileInput(input: HTMLInputElement): File[] {
  const files = Array.from(input.files ?? [])
  input.value = ''
  return files
}
