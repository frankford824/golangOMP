import {describe,it,expect} from 'vitest'
import {chinaDateInput,pricingWindow} from './cost-rule-timing'
describe('price schedule',()=>{
 it('uses China business time independently of browser timezone',()=>{expect(chinaDateInput('2026-09-23T00:00:00Z')).toBe('2026-09-23T08:00');expect(pricingWindow('2026-09-23T08:00','').effective_from).toBe('2026-09-23T08:00:00+08:00')})
 it('explicitly clears both boundaries and rejects inverted windows',()=>{expect(pricingWindow('','')).toEqual({effective_from:null,effective_to:null});expect(()=>pricingWindow('2026-09-24T08:00','2026-09-23T08:00')).toThrow()})
})
