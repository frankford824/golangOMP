// @vitest-environment node
import { readFileSync } from 'node:fs'
import { fileURLToPath } from 'node:url'
import { describe, expect, it } from 'vitest'

const componentSource = readFileSync(fileURLToPath(new URL('./ResourceGroupMaterialCard.vue', import.meta.url)), 'utf8')
const recipesSource = readFileSync(fileURLToPath(new URL('../../styles/recipes.css', import.meta.url)), 'utf8')
const tokensSource = readFileSync(fileURLToPath(new URL('../../styles/tokens.css', import.meta.url)), 'utf8')

describe('ResourceGroupMaterialCard style contract', () => {
  it('uses the shared asset-workbench recipe and only declared asset-workbench tokens', () => {
    expect(componentSource).not.toContain('<style')

    const recipeMarker = '/* Resource-group card and pinned client-publication dialog. */'
    const resourceRecipe = recipesSource.slice(recipesSource.indexOf(recipeMarker))
    expect(resourceRecipe).not.toContain('rgb(var(--aw-')

    const declaredTokens = new Set([...tokensSource.matchAll(/(--aw-[\w-]+)\s*:/g)].map((match) => match[1]))
    const referencedTokens = new Set([...resourceRecipe.matchAll(/var\((--aw-[\w-]+)/g)].map((match) => match[1]))
    expect([...referencedTokens].filter((token) => !declaredTokens.has(token))).toEqual([])
  })
})
