// The production OA resolver uses this module so demo handlers never enter its module graph.
export function isMockEnabled(): boolean { return false }
export function createMockAdapter(): never { throw new Error('Mock adapter unavailable in production') }
