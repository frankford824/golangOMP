export function validateWorkbenchPassword(password: string): string {
  if (password.trim().length < 8) return '密码至少8位，并包含字母和数字'
  if (!/[A-Za-z]/.test(password) || !/[0-9]/.test(password)) return '密码至少8位，并包含字母和数字'
  return ''
}
